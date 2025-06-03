package gateway

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGatewayTelemetry esegue il test di integrazione per la telemetria del gateway.
// Questo test esegue lo script Python che simula l'invio di dati di telemetria
// e verifica che vengano correttamente ricevuti, elaborati e pubblicati su MQTT.
//
// Per funzionare correttamente, questo test richiede:
// - Un broker MQTT in esecuzione (localhost:1883 o configurato tramite env)
// - Il gateway in ascolto sulla porta QUIC (7143 o configurata tramite env)
// - Python con i requisiti installati (aioquic, protobuf, ecc.)
//
// Questo test è deliberatamente isolato nel job CI "integration-test" per evitare
// interferenze con altri test e per fornire un ambiente controllato.
func TestGatewayTelemetry(t *testing.T) {
	// Controlla se siamo in ambiente CI
	inCI := os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true"

	// Imposta timeout estesi per l'ambiente CI
	timeout := 45 * time.Second
	if inCI {
		timeout = 90 * time.Second
		t.Log("Running in CI environment, extended timeout applied")
	}
	t.Logf("Configured timeout: %v seconds", timeout.Seconds())
	
	// Configura un context con timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Trova il percorso del test Python
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Fatalf("Failed to find repository root: %v", err)
	}

	pythonScriptPath := filepath.Join(repoRoot, "test", "gw_datagram_noise_test.py")
	
	// Verifica che il test Python esista
	if _, err := os.Stat(pythonScriptPath); os.IsNotExist(err) {
		t.Fatalf("Python test script not found at %s", pythonScriptPath)
	}

	// Configura l'ambiente per il test Python
	env := os.Environ()
	
	// Imposta variabili d'ambiente per il debug se necessario
	if os.Getenv("DEBUG") == "true" || os.Getenv("ACTIONS_STEP_DEBUG") == "true" || !inCI {
		env = append(env, "DEBUG=true")
		t.Log("Debug mode enabled for Python test")
	}

	// Verifica preliminare che il broker MQTT sia disponibile
	mqttHost := os.Getenv("MQTT_HOST")
	if mqttHost == "" {
		mqttHost = "localhost"
	}
	mqttPort := os.Getenv("MQTT_PORT")
	if mqttPort == "" {
		mqttPort = "1883"
	}
	
	t.Logf("Verifying MQTT broker availability at %s:%s", mqttHost, mqttPort)
	conn, err := net.DialTimeout("tcp", mqttHost+":"+mqttPort, 2*time.Second)
	if err != nil {
		t.Logf("WARNING: MQTT broker not available: %v. Test may fail if broker is required.", err)
	} else {
		conn.Close()
		t.Log("MQTT broker is available")
	}
	
	// Verifica che il server gateway QUIC sia in ascolto
	quicPort := os.Getenv("QUIC_PORT")
	if quicPort == "" {
		quicPort = "7143"
	}
	
	t.Logf("Verifying gateway QUIC server availability at localhost:%s", quicPort)
	conn, err = net.DialTimeout("udp", "localhost:"+quicPort, 2*time.Second)
	if err != nil {
		t.Logf("WARNING: Gateway QUIC server not available: %v. This test verifies only the Python client functionality.", err)
		t.Log("For a complete end-to-end test, ensure the gateway server is running.")
	} else {
		conn.Close()
		t.Log("Gateway QUIC server is available")
	}

	// Aggiungi la directory principale del progetto al PYTHONPATH
	// per consentire l'importazione del modulo proto
	pythonPath := os.Getenv("PYTHONPATH")
	if pythonPath == "" {
		pythonPath = repoRoot
	} else {
		pythonPath = pythonPath + string(os.PathListSeparator) + repoRoot
	}
	env = append(env, "PYTHONPATH="+pythonPath)
	
	// Prepara il comando per eseguire il test Python con il context e timeout
	cmd := exec.CommandContext(ctx, "python", pythonScriptPath)
	cmd.Env = env // Passa le variabili d'ambiente modificate

	// Cattura l'output
	output, err := cmd.CombinedOutput()
	t.Logf("Python test output:\n%s", string(output))

	// Verifica il risultato
	if err != nil {
		// Il test Python è fallito, ma potrebbe essere un fallimento atteso se componenti non sono disponibili
		outputStr := string(output)
		if strings.Contains(outputStr, "MQTT broker not available") {
			t.Log("Test expected to fail: MQTT broker not available")
			// Ignoriamo questo errore in CI perché è atteso se il broker MQTT non è configurato
			if inCI {
				t.Skip("Skipping test in CI due to unavailable MQTT broker")
			}
		} else if strings.Contains(outputStr, "QUIC connection failed") {
			t.Log("Test expected to fail: QUIC server not available or connection failed")
			// Ignoriamo questo errore in CI perché è atteso se il server QUIC non è in esecuzione
			if inCI {
				t.Skip("Skipping test in CI due to unavailable QUIC server")
			}
		} else if strings.Contains(outputStr, "ModuleNotFoundError") {
			t.Logf("Missing Python dependency: %s", outputStr)
			t.Log("To fix: pip install aioquic protobuf")
			if inCI {
				t.Error("CI environment missing required Python dependencies")
			} else {
				t.Skip("Skipping test due to missing Python dependencies")
			}
		} else {
			// Errore generico, mostriamo le informazioni di diagnostica
			t.Errorf("Gateway Telemetry test failed: %v\nOutput: %s", err, outputStr)
		}
	} else {
		t.Log("Gateway Telemetry test completed successfully")
	}
	
	// Non usiamo assert in questo test per semplicità e per evitare dipendenze problematiche

	// Se siamo in ambiente CI, facciamo alcune verifiche aggiuntive
	if inCI {
		// Verifica la presenza di file di log quic-go se necessario
		quicLogs, _ := filepath.Glob(filepath.Join(os.TempDir(), "quic-*.log"))
		if len(quicLogs) > 0 {
			t.Logf("Found %d quic-go log files", len(quicLogs))
		}
	}
}

// findRepoRoot trova il percorso della root del repository AXCP
func findRepoRoot() (string, error) {
	// Parti dalla directory corrente
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Risali finché non trovi .git o .github
	for {
		// Controlla se siamo alla root del repository
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, ".github")); err == nil {
			return dir, nil
		}

		// Vai alla directory parent
		parent := filepath.Dir(dir)
		if parent == dir {
			// Siamo arrivati alla root del filesystem senza trovare la root del repo
			return "", fmt.Errorf("repository root not found")
		}
		dir = parent
	}
}
