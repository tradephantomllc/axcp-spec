package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/tradephantom/axcp-spec/sdk/go/axcp"
	"github.com/tradephantom/axcp-spec/sdk/go/netquic"
	"github.com/tradephantom/axcp-spec/sdk/go/pb"
)

type Config struct {
	Gateway     string `json:"gateway" yaml:"gateway"`
	Profile     uint32 `json:"profile" yaml:"profile"`
	IntervalSec int    `json:"interval_sec" yaml:"interval_sec"`
}

func loadConfig(path string) Config {
	log.Printf("Lettura del file di configurazione: %s", path)
	b, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("Errore nella lettura del file: %v", err)
		return Config{Gateway: "127.0.0.1:7143", Profile: 3, IntervalSec: 5}
	}
	
	log.Printf("Contenuto file config: %s", string(b))
	
	var c Config
	err = json.Unmarshal(b, &c)
	if err != nil {
		log.Printf("Errore nel parsing JSON: %v. Provo con valori di default.", err)
		return Config{Gateway: "127.0.0.1:7143", Profile: 3, IntervalSec: 5}
	}
	
	log.Printf("Config dopo parsing: gateway=%s, profile=%d, interval=%d", c.Gateway, c.Profile, c.IntervalSec)
	
	// Valori di default se non specificati
	if c.Gateway == "" { c.Gateway = "127.0.0.1:7143" }
	if c.IntervalSec == 0 { c.IntervalSec = 5 }
	if c.Profile == 0 { c.Profile = 3 }
	
	return c
}

func sendHello(c *netquic.Client, profile uint32) error {
	traceID := "rpi-agent-hello-" + time.Now().Format("20060102-150405.000")
	env := axcp.NewEnvelope(traceID, profile)
	return c.SendEnvelope(env)
}

func buildTelemetry(profile uint32) *pb.TelemetryDatagram {
	cpuP, _ := cpu.Percent(0, false)
	vmem, _ := mem.VirtualMemory()
	
	// Creiamo un nuovo telemetry datagram usando il factory method del package pb
	td := pb.NewTelemetryDatagram()
	
	// Impostiamo il timestamp in millisecondi (convertendo da nano)
	td.TimestampMs = uint64(time.Now().UnixNano() / 1_000_000)
	
	// Creiamo un oggetto SystemStats per il payload
	sysStats := pb.NewSystemStats()
	sysStats.CpuPercent = uint32(cpuP[0])
	sysStats.MemBytes = vmem.Used
	
	// Associamo il SystemStats al TelemetryDatagram tramite il campo oneof
	td.Payload = &pb.TelemetryDatagram_System{
		System: sysStats,
	}
	
	return td
}

// Modalità di simulazione che mostra solo i dati locali senza inviarli al gateway
func runSimulation(cfg Config) {
	log.Printf("Esecuzione in modalità SIMULAZIONE - Nessuna connessione al gateway")
	log.Printf("Parametri: gateway=%s, profile=%d, interval=%ds", 
		cfg.Gateway, cfg.Profile, cfg.IntervalSec)
	
	log.Printf("Raccolta dati di telemetria ogni %d secondi...\n", cfg.IntervalSec)
	
	ticker := time.NewTicker(time.Duration(cfg.IntervalSec) * time.Second)
	for range ticker.C {
		td := buildTelemetry(cfg.Profile)
		log.Printf("[SIMULAZIONE] Telemetria generata: CPU %.1f%%, Memory %d bytes", 
			float64(td.GetSystem().GetCpuPercent()), td.GetSystem().GetMemBytes())
		log.Printf("[SIMULAZIONE] Timestamp: %d ms", td.TimestampMs)
	}
}

func main() {
	// In ambiente di sviluppo, prova prima a usare config.json locale, poi config.yaml, infine il percorso di default
	configPath := "config.json"
	_, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Printf("Config file non trovato in %s, provo con config.yaml", configPath)
		configPath = "config.yaml"
		_, err = ioutil.ReadFile(configPath)
		if err != nil {
			log.Printf("Config file non trovato in %s, usando il percorso di default", configPath)
			configPath = "/etc/axcp/config.yaml"
		}
	}

	log.Printf("Caricamento configurazione da %s", configPath)
	cfg := loadConfig(configPath)
	log.Printf("Configurazione caricata: gateway=%s, profile=%d, interval=%ds", 
		cfg.Gateway, cfg.Profile, cfg.IntervalSec)

	// Se esiste un file "simulation.flag" nella directory corrente, esegui in modalità simulazione
	_, err = ioutil.ReadFile("simulation.flag")
	if err == nil {
		runSimulation(cfg)
		return
	}

	log.Printf("Tentativo di connessione al gateway QUIC: %s", cfg.Gateway)
	log.Printf("Inizializzazione configurazione TLS insicura per sviluppo")
	tlsConf := netquic.InsecureTLSConfig()

	client, err := netquic.Dial(cfg.Gateway, tlsConf)
	if err != nil {
		log.Printf("ERRORE: Impossibile connettersi al gateway QUIC %s: %v", cfg.Gateway, err)
		log.Printf("\nPer eseguire in modalità simulazione (senza connessione al gateway):")
		log.Printf("1. Crea un file vuoto 'simulation.flag' nella directory corrente")
		log.Printf("2. Rilancia l'applicazione\n")
		log.Fatal("Terminazione applicazione a causa dell'errore di connessione")
	}
	defer client.Close()

	log.Printf("Connessione al gateway QUIC riuscita!")

	// Send "hello" via stream
	if err := sendHello(client, cfg.Profile); err != nil {
		log.Printf("AVVISO: Invio messaggio hello fallito: %v", err)
	} else {
		log.Printf("Messaggio hello inviato con successo")
	}

	log.Printf("Invio telemetria ogni %d secondi a %s con profilo %d", 
		cfg.IntervalSec, cfg.Gateway, cfg.Profile)

	ticker := time.NewTicker(time.Duration(cfg.IntervalSec) * time.Second)
	for range ticker.C {
		td := buildTelemetry(cfg.Profile)
		// Utilizziamo direttamente SendTelemetryWithProfile con profilo specifico
		if err := client.SendTelemetryWithProfile(td, cfg.Profile); err != nil {
			log.Printf("ERRORE: Invio telemetria fallito: %v", err)
		} else {
			log.Printf("Telemetria inviata con successo: CPU %.1f%%, Memory %d bytes", 
				float64(td.GetSystem().GetCpuPercent()), td.GetSystem().GetMemBytes())
		}
	}
}
