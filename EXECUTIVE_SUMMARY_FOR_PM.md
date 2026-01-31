# AXCP Strategic Restructuring: Executive Summary

**Per:** Product Manager AXCP
**Da:** Technical Analysis Team
**Data:** 2026-01-17
**Oggetto:** Proposta di ristrutturazione licensing e architettura prima del lancio pubblico

---

## Contesto

AXCP è stato creato e la repository è pubblica su GitHub, ma il lancio ufficiale non è ancora avvenuto. Questo ci dà l'opportunità di rivedere il modello di licensing e la struttura del prodotto prima di andare sul mercato.

---

## Il Problema con il Modello Attuale

### Struttura Corrente (BUSL-1.1)

```
AXCP-SPEC (BUSL-1.1 → Apache 2.0 nel 2029)
├── Profile 0: Basic (QUIC + TLS)
├── Profile 1: Secure-Lite (DID + Signatures)
├── Profile 2: Secure + Sync (Context-Sync, mTLS, DP opzionale)
└── [Profile 3 parziale]

AXCP-ENTERPRISE (Proprietario)
└── Profile 3: Enterprise-Privacy (DP mandatory, PII, TRI-AI)
```

### Tre Problemi Identificati

**1. BUSL-1.1 Frena l'Adozione**
- Molte aziende hanno policy che vietano licenze "quasi open source"
- Il messaggio "diventa Apache nel 2029" non convince — 4 anni sono troppi
- MCP (MIT) e A2A non hanno questa frizione

**2. Il Mercato Enterprise è Troppo Stretto**
- Con Profile 0-1-2 che diventano gratis nel 2029, solo chi ha bisogno di Profile 3 pagherebbe
- Profile 2 offre già sicurezza notevole (mTLS, DP opzionale, enclaves)
- Il bacino di utenti che *necessita* Profile 3 è limitato (solo regulated industries)

**3. Profile 0 da Solo NON è Production-Ready**
- Analizzando il codice, Profile 0 attualmente offre solo QUIC + TLS
- **Nessuna autenticazione peer** → chiunque può impersonare qualsiasi agente
- **Nessuna firma messaggi** → i messaggi possono essere falsificati
- **Nessuna replay protection** → comandi possono essere ripetuti da attaccanti
- Inutilizzabile per comunicazione remota su internet pubblico

---

## La Proposta: Open Core con 3 Tier

### Principio Guida

> **AXCP non è un'alternativa a MCP, A2A o ACP — è il layer di interoperabilità che li connette tutti.**

Questo posizionamento richiede che la versione open source sia **production-ready con sicurezza completa**, altrimenti non può fungere da bridge credibile tra protocolli.

### Nuova Struttura

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           NUOVA STRUTTURA AXCP                               │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                    AXCP CORE (Apache 2.0 - GRATIS)                  │    │
│  │                    ─────────────────────────────────                │    │
│  │                                                                     │    │
│  │  Profile 0 + Profile 1 UNIFICATI                                   │    │
│  │                                                                     │    │
│  │  Transport:           Security:                                    │    │
│  │  • QUIC + Protobuf    • DID mutual authentication (ECDH)          │    │
│  │  • TLS 1.3            • Ed25519 message signatures                │    │
│  │  • QUIC Datagrams     • Signature verification                    │    │
│  │                       • Replay attack protection                  │    │
│  │                       • Profile negotiation                       │    │
│  │                                                                     │    │
│  │  Protocol:            Interoperability:                           │    │
│  │  • Capability neg.    • MCP ↔ AXCP gateway                        │    │
│  │  • Tool discovery     • A2A ↔ AXCP gateway                        │    │
│  │  • Telemetry          • ACP ↔ AXCP gateway                        │    │
│  │  • Error handling     • REST API bridging                         │    │
│  │                                                                     │    │
│  │  ✅ PRODUCTION-READY per uso remoto su internet pubblico          │    │
│  │  ✅ Completo per orchestrazione multi-agente                       │    │
│  │  ✅ Sicuro per bridging tra ecosistemi                             │    │
│  │                                                                     │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                 AXCP ADVANCED (Commercial - $)                      │    │
│  │                 ──────────────────────────────                      │    │
│  │                                                                     │    │
│  │  Tutto del Core, più Profile 2:                                    │    │
│  │                                                                     │    │
│  │  • Context-Sync con delta patches (CRDT)                          │    │
│  │  • mTLS client certificates                                       │    │
│  │  • SGX/SEV enclave support (opzionale)                            │    │
│  │  • Differential Privacy (opzionale)                               │    │
│  │  • Rate limiting avanzato                                         │    │
│  │  • Structured logging                                             │    │
│  │  • Email support                                                  │    │
│  │                                                                     │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                    │                                         │
│                                    ▼                                         │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                AXCP ENTERPRISE (Commercial - $$)                    │    │
│  │                ─────────────────────────────────                    │    │
│  │                                                                     │    │
│  │  Tutto dell'Advanced, più Profile 3:                               │    │
│  │                                                                     │    │
│  │  • Differential Privacy MANDATORY                                  │    │
│  │  • PII filtering & redaction                                      │    │
│  │  • Compliance reporting (GDPR, HIPAA, SOC2)                       │    │
│  │  • Audit trails (Merkle tree verified)                            │    │
│  │  • TRI-AI Orchestration System                                    │    │
│  │  • Dedicated support + SLA                                        │    │
│  │                                                                     │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Perché Unificare Profile 0 e Profile 1?

### Opzione Scartata: Profile 0 "Enhanced" con PSK/API Keys

Avevamo considerato di aggiungere autenticazione "custom" (PSK, API keys) al Profile 0 per renderlo usabile. **Abbiamo scartato questa opzione** perché:

1. **Devia dallo spec** — creerebbe una versione "bastardizzata" del protocollo
2. **Non interoperabile** — altri nodi AXCP si aspettano Profile 1 per auth
3. **Posizionamento debole** — sembrerebbe un "demo crippled"
4. **Sforzo simile** — implementare PSK richiede quasi lo stesso tempo di DID

### Opzione Scelta: Profile 1 Incluso nel Core

1. **Spec-compliant** — nessuna deviazione, nessuna confusione
2. **Production-ready** — funziona su internet pubblico out of the box
3. **Interoperabile** — gateway può fare bridge sicuri con MCP/A2A/ACP
4. **Credibilità** — DIDComm v2 è uno standard W3C, Ed25519 è industry standard
5. **Code reuse** — Ed25519 è già implementato in enterprise, basta spostarlo

---

## Perché Questo Modello Funziona Meglio

### Per l'Adozione

| Aspetto | Modello Attuale | Nuovo Modello |
|---------|-----------------|---------------|
| Licenza Core | BUSL-1.1 (frizioni) | Apache 2.0 (zero frizioni) |
| Usabilità Core | Solo locale/testing | Production-ready |
| Competitività | Svantaggiato vs MCP/A2A | Alla pari + interop |
| Messaggio | "Diventa free nel 2029" | "È free e completo ORA" |

### Per la Monetizzazione

| Aspetto | Modello Attuale | Nuovo Modello |
|---------|-----------------|---------------|
| Cosa si paga | Solo Profile 3 (nicchia) | Profile 2 + Profile 3 |
| Valore percepito | "Pago per compliance" | "Pago per sync + privacy" |
| Mercato | Solo regulated industries | Qualsiasi enterprise serio |
| Upsell path | Confuso (4 livelli) | Chiaro (3 livelli) |

### Feature Comparison

| Feature | Core (Free) | Advanced ($) | Enterprise ($$) |
|---------|-------------|--------------|-----------------|
| QUIC + TLS 1.3 | ✅ | ✅ | ✅ |
| DID authentication | ✅ | ✅ | ✅ |
| Ed25519 signatures | ✅ | ✅ | ✅ |
| Replay protection | ✅ | ✅ | ✅ |
| MCP/A2A/ACP gateway | ✅ | ✅ | ✅ |
| Context-Sync | ❌ | ✅ | ✅ |
| mTLS | ❌ | ✅ | ✅ |
| Differential Privacy | ❌ | Optional | Mandatory |
| PII filtering | ❌ | ❌ | ✅ |
| Audit trails | ❌ | ❌ | ✅ |
| TRI-AI | ❌ | ❌ | ✅ |

---

## Cosa Cambia nel Codice

### Spostamenti Principali

**Da axcp-enterprise → axcp-spec (Core):**
```
enterprise/secure/telemetry/signer.go → sdk/go/auth/signer.go
(Ed25519 già implementato, lo spostiamo nel Core)
```

**Da axcp-spec → axcp-enterprise:**
```
edge/gateway/internal/dp_*.go → advanced/dp/
sdk/go/dp/ → advanced/dp/sdk/
config/dp_budget.yaml → advanced/config/
tests/dp/ → advanced/tests/
```

### Nuovo Codice da Scrivere

```
axcp-spec/sdk/go/auth/
├── did.go        # DID verification (NUOVO - ~3 giorni)
├── signer.go     # Ed25519 (ESISTENTE - da enterprise)
├── verifier.go   # Signature verification (NUOVO - ~1 giorno)
└── replay.go     # Replay protection (NUOVO - ~1 giorno)

axcp-spec/sdk/go/negotiate/
└── profile.go    # Profile negotiation (NUOVO - ~2 giorni)
```

### Riorganizzazione Enterprise

```
axcp-enterprise/
├── advanced/           # Profile 2 (NUOVA struttura)
│   ├── context-sync/   # Delta patches, CRDT
│   ├── mtls/           # mTLS middleware
│   ├── dp/             # DP opzionale (spostato da spec)
│   └── enclave/        # SGX/SEV
│
├── enterprise/         # Profile 3 (esistente, riorganizzato)
│   ├── secure/         # Telemetry, PII filter
│   ├── dp-mandatory/   # DP enforcement
│   ├── audit/          # Audit trails
│   └── compliance/     # GDPR/HIPAA
│
└── tri-ai/             # TRI-AI (spostato qui)
    ├── gemini-cli-agent/
    ├── codex-cli-agent/
    └── claude-code-agent/
```

---

## Timeline Proposta

```
┌─────────────────────────────────────────────────────────────────┐
│                    TIMELINE: 10 SETTIMANE                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  FASE 1: Core Authentication                    [Week 1-2]     │
│  ─────────────────────────────                                  │
│  • Spostare signer.go da enterprise a Core                     │
│  • Implementare DID verification                               │
│  • Implementare replay protection                              │
│  • Implementare profile negotiation                            │
│  • Test suite per auth                                         │
│                                                                 │
│  FASE 2: Code Separation                        [Week 3]       │
│  ───────────────────────                                        │
│  • Spostare DP modules in enterprise/advanced                  │
│  • Riorganizzare struttura enterprise                          │
│  • Aggiornare import paths                                     │
│  • Aggiornare CI/CD                                            │
│                                                                 │
│  FASE 3: Gateway Update                         [Week 4]       │
│  ──────────────────────                                         │
│  • Rimuovere DP dal gateway Core                               │
│  • Integrare DID auth nel gateway                              │
│  • Aggiornare MCP/A2A bridge                                   │
│                                                                 │
│  FASE 4: Advanced Tier                          [Week 5-6]     │
│  ────────────────────                                           │
│  • Implementare Context-Sync completo                          │
│  • Aggiungere mTLS middleware                                  │
│  • Integrare DP opzionale                                      │
│  • Test suite Advanced                                         │
│                                                                 │
│  FASE 5: Enterprise Polish                      [Week 7-8]     │
│  ───────────────────────                                        │
│  • DP enforcement mandatory                                    │
│  • Compliance reporting                                        │
│  • Audit trails enhancement                                    │
│  • Multi-tenant isolation                                      │
│                                                                 │
│  FASE 6: Documentation & Launch                 [Week 9-10]    │
│  ──────────────────────────────                                 │
│  • Cambiare LICENSE a Apache 2.0                               │
│  • Nuovo README (enfasi interoperabilità)                      │
│  • Getting started guide                                       │
│  • Documentazione authentication                               │
│  • Aggiornare spec a v1.0                                      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Domande per il PM

Prima di procedere, vorremmo il tuo feedback su:

1. **Posizionamento**: Sei d'accordo che AXCP debba essere posizionato come "interoperability layer" piuttosto che "alternativa a MCP/A2A"?

2. **Profile 0+1 unificati**: Condividi la scelta di includere DID + Ed25519 nel Core gratuito? L'alternativa (PSK/API keys custom) ci sembrava tecnicamente inferiore.

3. **3 Tier vs 4 Tier**: Pensi che 3 tier (Core/Advanced/Enterprise) sia più chiaro di 4 tier (Basic/Standard/Advanced/Enterprise)?

4. **Context-Sync a pagamento**: Nella nuova struttura, Context-Sync è in Advanced (a pagamento). Questo potrebbe limitare alcuni use case. Preferiresti includerlo nel Core?

5. **Timeline 10 settimane**: Ti sembra realistica? Ci sono priorità che vorresti cambiare?

6. **Spec v1.0**: Dovremmo aggiornare lo spec per riflettere l'unificazione Profile 0+1, o mantenerli separati nel documento e unificarli solo nell'implementazione?

7. **Naming**: "AXCP Core / Advanced / Enterprise" ti sembra chiaro? Alternative considerate: "Community/Pro/Enterprise", "Open/Business/Enterprise"

8. **Altro**: Ci sono aspetti del progetto originale che ritieni debbano essere preservati o che questa ristrutturazione potrebbe compromettere?

---

## Prossimi Passi

Dopo aver ricevuto il tuo feedback:

1. Finalizzare il piano con eventuali modifiche
2. Iniziare Phase 1 (Core Authentication)
3. Procedere in modo incrementale con review a ogni fase

---

**Documento completo disponibile in:** `axcp-spec/RESTRUCTURING_PLAN.md`

