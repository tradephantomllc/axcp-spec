# AXCP Restructuring: Operational Roadmap

**Version:** 2.0
**Date:** 2026-01-29
**Status:** APPROVED - In Execution
**Created by:** PM
**Execution Team:** Claude Code, Codex CLI, Human Lead

---

## Obiettivo del Ciclo

* Trasformare AXCP Core in "Secure Baseline" sotto Apache 2.0, con confini puliti.
* Spostare Sync + DP + Compliance in Advanced/Enterprise (commerciale) con dependency direction rigorosa.
* Arrivare a rilascio Core v1.0.0 (tag) con repo nuovamente pubblica.

---

## Legenda Stati

| Stato | Significato |
|-------|-------------|
| **DONE** | Completato e verificato |
| **IN PROGRESS** | Attualmente in lavorazione |
| **NEXT** | Prossimo nella sequenza |
| **PENDING** | Da fare, in attesa di dipendenze |
| **VERIFY** | Da confermare (non evidenza diretta) |
| **OPTIONAL** | Separato/non-bloccante per release |

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
M0 → M1 → M2 → M3 → M4 → M6 → M7
                    ↓
                   M5 (parallelo, OPTIONAL)
```

---

## A) Governance / Legal / Repo Safety

### M0.1 — Freeze main + branch di lavoro + protezioni
**Status:** DONE

- [x] main protetto
- [x] Lavoro su `restructure/open-core`

### M0.2 — Licensing switch Core → Apache 2.0
**Status:** DONE

- [x] LICENSE file updated to Apache 2.0
- [x] NOTICE file present

### M0.3 — Trademark policy minimale
**Status:** DONE

- [x] TRADEMARK.md exists
- [x] Linked in README.md

---

## B) Core Layout + CI Boundaries (axcp-spec)

### M1.1 — Nuova struttura directory (placeholder, senza spostare codice)
**Status:** DONE

- [x] Directory structure reorganized (spec/, sdk/, docs/, etc.)

### M1.2 — CI aggiornata per nuovo layout (Core standalone)
**Status:** DONE

- [x] axcp-spec CI all green
- [x] Cross-repo integration working

---

## C) Core Secure Baseline (axcp-spec)

### M2.1 — Ed25519 signer/verifier in Core
**Status:** DONE

- [x] `sdk/go/auth/ed25519.go` implemented

### M2.2 — DID mutual authentication in Core
**Status:** DONE

- [x] `sdk/go/auth/did.go` implemented
- [x] Transcript format: `AXCP-DID-AUTH-v1`

### M2.3 — Replay protection in Core
**Status:** DONE

- [x] `sdk/go/auth/replay.go` implemented
- [x] Sequence + nonce modes supported

### M2.4 — Profile negotiation secure-baseline-v1
**Status:** DONE

- [x] `sdk/go/negotiate/profile.go` implemented
- [x] `secure-baseline-v1` as production default
- [x] `transport-only-v0` deprecated

---

## D) DP fuori dal Core + Split Advanced/Enterprise (multi-repo)

### M3.1 — Rimozione DP dal Core (proto/spec/sdk/gateway)
**Status:** DONE

- [x] DP lives in axcp-advanced, not in Core
- [x] Core is Apache 2.0 / public

### M3.x.1 — Creazione repo axcp-advanced
**Status:** DONE

- [x] `github.com/tradephantomllc/axcp-advanced` created

### M3.x.2 — Migrazione DP in axcp-advanced
**Status:** DONE

- [x] DP module in `axcp-advanced/dp/`
- [x] Tests passing

### M3.x.3 — Enterprise rewiring: enterprise importa DP da advanced
**Status:** DONE

- [x] `axcp-enterprise` imports from `axcp-advanced`
- [x] PR #4 merged with all checks green

### M3.x.4 — Cross-repo CI integration (Core + Advanced)
**Status:** DONE

- [x] Enterprise CI checks out axcp-advanced
- [x] Stack integration tests passing

### M3.x.5 — Delivery policy GitHub Teams (advanced-customers / enterprise-customers)
**Status:** PENDING (separato, non-bloccante per release)

- [ ] GitHub Teams configuration
- [ ] Stripe → team assignment automation
- [ ] Revoca e onboarding clienti

---

## E) Gateway + Bridging Authenticated (axcp-spec)

### M4.1 — Enforce DID + signature + replay in gateway
**Status:** DONE

- [x] `gateway: integrate DID + Ed25519 + replay auth verification (M4.1) (#165)`

### M4.2 — Hardening bridges + examples authenticated_chat
**Status:** DONE

- [x] `bridge: add Secure Baseline auth to MCP bridge + authenticated_chat example (M4.2) (#166)`

---

## F) Spec + Docs (axcp-spec)

### M6.1 — Spec v1.0 "Secure Baseline" allineata al Core
**Status:** DONE

- [x] `spec/axcp-v1.0.md` created
- [x] Profile negotiation documented
- [x] DID authentication with transcript format
- [x] Replay protection semantics
- [x] Core vs Advanced/Enterprise boundaries defined
- [x] Old specs (v0.1, v0.2, v0.3) marked as superseded
- [x] README updated with spec link
- [x] Commit: `8bc30d3`

### M6.2 — Docs Core (authentication.md, gateway-setup.md, getting-started.md)
**Status:** NEXT

- [ ] Developer Getting Started guide
- [ ] Architecture diagrams
- [ ] API reference
- [ ] Migration guide from v0.x

---

## G) Release Readiness + Pubblicazione (axcp-spec)

### M7.1 — Audit "No enterprise code in Core" + license checks + boundary report
**Status:** PENDING

- [ ] Automated audit script
- [ ] No DP/sync/compliance in Core
- [ ] No imports to advanced/enterprise
- [ ] License headers consistent
- [ ] Boundary report attached to final PR

### M7.2 — Merge restructure/open-core → main + repo public + tag v1.0.0
**Status:** PENDING

- [ ] Final merge to main
- [ ] Tag v1.0.0
- [ ] Changelog updated
- [ ] Repo made public

---

## H) Enterprise Housekeeping (solo dopo Core pronto)

### M5.1 — Reorg tri-ai/* (repo enterprise)
**Status:** OPTIONAL / PENDING

- [ ] Directory reorganization in enterprise repo

### M5.2 — Advanced vs Enterprise separazione funzionale
**Status:** OPTIONAL / PENDING

- [ ] DP optional (Advanced) vs mandatory (Enterprise)
- [ ] Compliance features (Enterprise only)
- [ ] Clear packaging boundaries

---

## Critical Path to v1.0.0 Release

```
M6.1 (DONE) → M6.2 (NEXT) → M7.1 → M7.2
```

**Non-blocking items:**
- M3.x.5 (Teams/Delivery) - can happen post-release
- M5.1/M5.2 (Enterprise housekeeping) - OPTIONAL

---

## Impact / Feasibility / TTM Table

| Blocco                            | Impatto | Fattibilità | TTM |
| --------------------------------- | ------: | ----------: | --: |
| M6.1 Spec v1.0                    |       9 |           9 |   8 |
| M6.2 Docs Core                    |       8 |           9 |   8 |
| M7.1 Audit boundary               |      10 |           8 |   7 |
| M7.2 Release v1.0.0               |      10 |           8 |   6 |
| M3.x.5 Delivery Teams             |       7 |           9 |   8 |
| M5.1/M5.2 Enterprise housekeeping |       6 |           8 |   6 |

---

*Last updated: 2026-01-29 by Claude Code*
