# AXCP Restructuring: Operational Roadmap

**Version:** 1.0
**Date:** 2026-01-17
**Status:** APPROVED - Ready for Execution
**Created by:** PM
**Execution Team:** Claude Code, Codex CLI, Human Lead

---

## Obiettivo del Ciclo

* Trasformare AXCP Core in "Secure Baseline" sotto Apache 2.0, con confini puliti.
* Spostare Sync + DP + Compliance in Advanced/Enterprise (commerciale) con dependency direction rigorosa.
* Arrivare a rilascio Core v1.0.0 (tag) con repo nuovamente pubblica.

---

## Regole di Esecuzione (Vincolanti)

| Regola | Descrizione |
|--------|-------------|
| **Branch** | Tutto il lavoro su branch: `restructure/open-core` |
| **PR Policy** | PR piccole, 1 tema per PR, merge solo con CI 100% green |
| **Dependency Direction** | Core non importa MAI da Advanced/Enterprise; Advanced importa Core; Enterprise importa Core+Advanced |
| **No Cross-Module Internal** | Niente `internal/` cross-module: se serve condividere, si crea un package pubblico |

---

## Dipendenze Principali (Ordine Minimo)

```
M0 → M1 → M2 → M3 → M4 → (M5 parallelo dopo M4 stabile) → M6 → M7
```

---

## M0 — Pre-work Legale + Governance (Blocco Iniziale)

### Issue M0.1 — Freeze main + repo privata + branch di lavoro

**Task:**
- [ ] Impostare main frozen (solo hotfix critici)
- [ ] Mettere repo privata durante il refactor
- [ ] Creare branch `restructure/open-core`
- [ ] Abilitare policy "PR required" sul branch di lavoro (se non già)

**Acceptance Criteria:**
- Nessun commit diretto su `restructure/open-core` (solo via PR)
- Repo privata confermata

**Test/CI:**
- Nessun cambio codice richiesto, solo verifica pipeline esistente

---

### Issue M0.2 — Licensing switch (Core → Apache 2.0) + audit compatibilità

**Task:**
- [ ] Sostituire LICENSE con Apache 2.0
- [ ] Aggiornare README/CONTRIBUTING/CLA (testo compatibile con Apache 2.0)
- [ ] Audit dipendenze Core: tutte compatibili con Apache 2.0

**Acceptance Criteria:**
- Tutti i file legali aggiornati e coerenti
- Nota di transizione in CHANGELOG

**Test/CI:**
- CI green invariata
- (Se avete tooling) job "license-check" o checklist manuale

---

### Issue M0.3 — Trademark policy minimale

**Task:**
- [ ] Creare TRADEMARK.md (regole base su uso del nome/logo "AXCP")

**Acceptance Criteria:**
- Documento presente e linkato da README

**Test/CI:**
- N/A (doc-only)

---

## M1 — Stabilizzazione Layout Repo + CI Boundaries

### Issue M1.1 — Applicare la nuova struttura directory (vuota, senza spostare codice ancora)

**Task:**
- [ ] Creare le directory finali previste:
  - `sdk/go/auth`
  - `sdk/go/negotiate`
  - `gateway/telemetry`
  - `docs/*`
  - `spec/*`
  - `examples/*`
- [ ] Lasciare placeholder dove necessario

**Acceptance Criteria:**
- Repo riflette la struttura finale senza breaking changes

**Test/CI:**
- Tutti i test attuali passano

---

### Issue M1.2 — Aggiornare CI per supportare il nuovo layout (senza logica enterprise)

**Task:**
- [ ] Aggiornare path/working-directory dei job se necessari
- [ ] Garantire: "Core CI runs standalone"

**Acceptance Criteria:**
- Pipeline completa verde su `restructure/open-core`

**Test/CI:**
- Tutti i job attuali green

---

## M2 — Core Secure Baseline (Estrazione + Implementazione Auth)

### Issue M2.1 — Portare Ed25519 signer/verifier in Core come package pubblico

**Task:**
- [ ] Creare `sdk/go/auth/{signer.go,verifier.go}`
- [ ] Spostare/copiarne il codice "buono" dall'Enterprise (senza trascinare dipendenze enterprise)
- [ ] Aggiungere unit test minimi (sign/verify, tamper, empty payload)

**Acceptance Criteria:**
- Package `sdk/go/auth` utilizzabile da Core e (in futuro) da Enterprise via import pubblico
- Nessun uso di internal packages condivisi

**Test/CI:**
- `go test ./...` green

---

### Issue M2.2 — DID mutual authentication (Core)

**Task:**
- [ ] Implementare `sdk/go/auth/did.go` (verifica DID, risoluzione/validazione essenziale, handshake constraints)
- [ ] Definire interfacce per DID resolver (mockabile) per non legarsi a un provider
- [ ] Unit test con resolver finto

**Acceptance Criteria:**
- DID flow verificabile in test senza rete

**Test/CI:**
- `go test ./...` green

---

### Issue M2.3 — Replay protection (Core)

**Task:**
- [ ] Implementare `sdk/go/auth/replay.go` (nonce/sequence window, TTL, cache strategy)
- [ ] Unit test: replay detect, expiry, window edges

**Acceptance Criteria:**
- Rejection deterministica dei replay

**Test/CI:**
- `go test ./...` green

---

### Issue M2.4 — Profile negotiation (Core "Secure Baseline")

**Task:**
- [ ] Implementare `sdk/go/negotiate/profile.go`
- [ ] Deprecare "Profile-0 transport-only" nel testo spec (non necessariamente rimuovere subito tutto il codice, ma deve risultare non-production)

**Acceptance Criteria:**
- Negotiation coperta da unit test

**Test/CI:**
- `go test ./...` green

---

## M3 — Rimozione DP dal Core + Pulizia Proto/Spec

### Issue M3.1 — Eliminare qualsiasi riferimento DP dal Core (proto/spec/sdk/gateway)

**Task:**
- [ ] Rimuovere DP refs da `proto/axcp.proto` (o separarle in modo che Core non le compili/usino)
- [ ] Eliminare/neutralizzare import o codice DP nel gateway Core

**Acceptance Criteria:**
- Core builda e testa senza DP
- Nessun file DP rimasto nel perimetro Core

**Test/CI:**
- `go test ./...` green
- gateway tests green

---

### Issue M3.2 — Migrazione file DP verso axcp-enterprise (repo privata)

**Task:**
- [ ] Spostare `dp_noise`, `dp_budget`, `config`, `sdk/go/dp`, `tests/dp` nel layout enterprise (`advanced/dp/*`)
- [ ] Aggiornare import path e moduli (Advanced dipende da Core)

**Acceptance Criteria:**
- Advanced builda con Core come dependency

**Test/CI:**
- (Nel repo enterprise) pipeline separata: Core tests + Advanced tests

---

## M4 — Gateway Core "Authenticated Bridging"

### Issue M4.1 — Integrare DID + signature verification nel gateway

**Task:**
- [ ] Enforce: messaggi entranti firmati, verifica Ed25519, replay protection
- [ ] Gestione errori e retry envelopes (Core)

**Acceptance Criteria:**
- Gateway rifiuta messaggi non autenticati quando Secure Baseline attivo

**Test/CI:**
- Test gateway telemetry + integrazione green

---

### Issue M4.2 — Hardening bridging MCP ↔ AXCP ↔ A2A ↔ ACP (Core)

**Task:**
- [ ] Assicurare che i bridge rispettino negotiation + auth
- [ ] Aggiornare esempi (`authenticated_chat`)

**Acceptance Criteria:**
- Esempio end-to-end funzionante (testabile in CI o come smoke test)

**Test/CI:**
- check examples + eventuale smoke test

---

## M5 — Enterprise Repo Re-org (Solo Dopo che Core è Stabile)

### Issue M5.1 — Reorg tri-ai/*

**Task:**
- [ ] Spostare agenti in:
  - `tri-ai/gemini-cli-agent`
  - `tri-ai/codex-cli-agent`
  - `tri-ai/claude-code-agent`

**Acceptance Criteria:**
- Build/test enterprise invariati

**Test/CI:**
- Enterprise pipeline green

---

### Issue M5.2 — Separazione Advanced vs Enterprise (dp optional vs mandatory + compliance)

**Task:**
- [ ] Advanced: sync + optional DP + mTLS + enclaves optional
- [ ] Enterprise: DP mandatory + audit Merkle + compliance reporting

**Acceptance Criteria:**
- Dependency direction rispettata, moduli separati

**Test/CI:**
- Advanced pipeline green
- Enterprise pipeline green

---

## M6 — Spec + Docs (Allineamento Totale al Nuovo Modello)

### Issue M6.1 — Spec v1.0 "Secure Baseline" (unifica Profile 0+1)

**Task:**
- [ ] Aggiornare `spec/axcp-v1.0.md`: nomenclatura Core/Advanced/Enterprise, deprecazioni chiare
- [ ] Aggiornare appendix interop

**Acceptance Criteria:**
- Spec consistente con implementazione Core

**Test/CI:**
- Doc-only, ma CI green

---

### Issue M6.2 — Docs Core: authentication.md + gateway-setup.md + getting-started.md

**Task:**
- [ ] Documentare: DID auth, signing, replay, negotiation, bridging

**Acceptance Criteria:**
- Percorso "hello world" + "authenticated chat" completo

**Test/CI:**
- Doc-only, CI green

---

## M7 — Release Readiness + Merge Finale

### Issue M7.1 — "No enterprise code in Core" verification

**Task:**
- [ ] Audit automatico o checklist: nessun file enterprise, nessun ref a DP/sync/compliance
- [ ] Verifica licenze

**Acceptance Criteria:**
- Report audit allegato alla PR finale

**Test/CI:**
- Full CI green

---

### Issue M7.2 — PR finale: restructure/open-core → main + repo pubblica + tag v1.0.0

**Task:**
- [ ] Merge su main
- [ ] Riportare repo pubblica
- [ ] Tag release v1.0.0 + changelog

**Acceptance Criteria:**
- Main verde, release taggata, repo pubblica

**Test/CI:**
- Full CI green post-merge

---

## Assegnazione Consigliata (Operativa)

| Ruolo | Responsabilità |
|-------|----------------|
| **PM** | Definizione issue, acceptance criteria, review finale PR |
| **Claude Code** | Implementazione Go (auth, replay, negotiate), update gateway, test fixes |
| **Codex CLI** | Refactor meccanici, spostamenti file, aggiornamento import, update docs scaffolding, CI YAML |
| **Human Lead** | Decisioni finali su legal/trademark, merge/branch policy, review sicurezza |

---

## Quick Reference: Issue Summary

| Milestone | Issues | Focus |
|-----------|--------|-------|
| **M0** | M0.1, M0.2, M0.3 | Legal + Governance |
| **M1** | M1.1, M1.2 | Repo Layout + CI |
| **M2** | M2.1, M2.2, M2.3, M2.4 | Core Auth Implementation |
| **M3** | M3.1, M3.2 | DP Removal + Migration |
| **M4** | M4.1, M4.2 | Gateway Auth Integration |
| **M5** | M5.1, M5.2 | Enterprise Reorg |
| **M6** | M6.1, M6.2 | Spec + Docs |
| **M7** | M7.1, M7.2 | Release + Go Public |

**Total Issues:** 17

---

## Next Steps

1. ✅ Roadmap saved
2. ⬜ Create issues M0.1–M0.3 (immediate)
3. ⬜ Create issues M1.1–M1.2 (queue)
4. ⬜ Begin execution with M0.1

---

**Document Status:** APPROVED - Execution Ready
