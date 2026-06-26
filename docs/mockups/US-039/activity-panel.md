# US-039 — Feed attività cross-prodotto · Mockup ASCII

**Epic:** EP-006 — Deployment Visibility & Audit  
**Componente:** pannello "Attività live" aggiunto ad `AdminPage.tsx`  
**Endpoint:** `GET /api/v1/admin/activity` → ultimi 10 deployment su tutti i prodotti

---

## Layout della pagina Admin con il nuovo pannello

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────┐
│ ░ SIDEBAR          │ AREA PRINCIPALE                                                            │
│ ─────────────────  │ ─────────────────────────────────────────────────────────────────────────  │
│  ⬡ KubeGate v1.0  │                                                                            │
│  ──────────────── │  All Products — Admin                           [US-039]                   │
│                    │  Platform-wide product inventory. Tutti i deployment recenti su tutti       │
│  Workspace         │  i prodotti, in tempo reale.                                               │
│  □ Products    4   │                                                                            │
│  ⊙ Deployments    │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│                    │  │ ◈ TEAL LEFT  │  │ ● BLU LEFT   │  │ ● VERDE LEFT │  │ ◈ ARANCIO    │   │
│  Platform          │  │              │  │              │  │              │  │              │   │
│  ○ Users & RBAC   │  │     4        │  │     11       │  │     3        │  │     3        │   │
│  ● Admin Dash ←   │  │ Tot. prodotti│  │ Tot. ambienti│  │ Con deploy   │  │ Prod envs    │   │
│  ○ Settings       │  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘   │
│                    │                                                                            │
│  ──────────────── │  ┌──────────────────────────────────────────────────────────────────────┐  │
│  SB Sara Bianchi   │  │ Prodotti                  4 prodotti    [  🔍 Cerca prodotti…      ] │  │
│  ⭐ DevOps Admin   │  │ ─────────────────────────────────────────────────────────────────── │  │
│                    │  │ NOME ↑              AMBIENTI        ULTIMO DEPLOY        [AZIONI]   │  │
│                    │  │ PA Platform API     ●●●  3          ✓ 14 Giu  v1.14.1   [Gestisci] │  │
│                    │  │ CA Customer App     ●●●  3          ✓ 10 Giu  v3.2.0    [Gestisci] │  │
│                    │  │ DS Data Sync        ●●●● 4          ⚠ 28 Mag  v0.9.3    [Gestisci] │  │
│                    │  │ NG Nexus Gateway    ●    1          —  mai               [Gestisci] │  │
│                    │  └──────────────────────────────────────────────────────────────────────┘  │
│                    │                                                                            │
│                    │  ┌──────────────────────────────────────────────────────────────────────┐  │
│                    │  │ ◈ Attività live                          Ultimi 10 eventi            │  │
│                    │  │ ─────────────────────────────────────────────────────────────────── │  │
│                    │  │                        ← NUOVO PANNELLO US-039 →                    │  │
│                    │  │  [vedere sezione dettagliata sotto]                                  │  │
│                    │  └──────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Pannello "Attività live" — Dettaglio righe

Il pannello è posizionato **sotto la tabella prodotti**, a tutta larghezza.  
Ogni riga rappresenta un evento di deployment ordinato per `deployed_at DESC`.

### Anatomia di una riga

```
┌───────────────────────────────────────────────────────────────────────────────────────────────┐
│ ◈ Attività live                                                        Ultimi 10 eventi       │
│ ──────────────────────────────────────────────────────────────────────────────────────────── │
│                                                                                               │
│  AVATAR  DESCRIZIONE EVENTO                                    PRODOTTO / COMPONENTE  QUANDO  │
│  ──────  ──────────────────────────────────────────────────   ─────────────────────  ─────── │
│                                                                                               │
│  [SB]    Sara B. ha rilasciato v1.15.0 → api-gateway / prod   platform-api           adesso  │
│   ●      ↑ punto verde PULSANTE = in_progress                  ↑ slug mono            ↑ rel.  │
│                                                                                               │
│  [MR]    Marco R. ha rilasciato v3.2.1 → frontend / prod       customer-app           2m fa   │
│   ●      ↑ punto verde SOLIDO = success                                                       │
│                                                                                               │
│  [LC]    Laura C. ha rilasciato v0.9.4 → worker / staging      data-sync              18m fa  │
│   ●      ↑ punto ROSSO = failure      [msg errore opzionale]                                  │
│                                                                                               │
│  ...     (fino a 10 righe totali)                                                             │
└───────────────────────────────────────────────────────────────────────────────────────────────┘
```

### Tre stati del pallino di stato

```
  ●  verde + ring pulsante  →  in_progress   (animazione pulse CSS)
  ●  verde solido           →  success       (glow statico: box-shadow verde)
  ●  rosso solido           →  failure       (glow statico: box-shadow rosso)
```

### Riga espansa con tutti gli elementi

```
┌───────────────────────────────────────────────────────────────────────────────────────┐
│  ●  [SB]  Sara Bianchi ha rilasciato  v1.15.0  →  api-gateway / production           │
│  ↑         ↑              ↑              ↑    ↑        ↑             ↑                │
│  status  avatar       actor_display   tag  freccia  component_name  environment_name  │
│  dot      initials       _name        mono         mono             mono              │
│                                                                                       │
│                                                  platform-api          adesso         │
│                                                      ↑                    ↑           │
│                                                  product_slug      timestamp relativo  │
│                                                  (link cliccabile)  (deployed_at)     │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

### Stato: failure con messaggio di errore

```
┌───────────────────────────────────────────────────────────────────────────────────────┐
│  ●  [LC]  Laura C. ha rilasciato  v0.9.4  →  worker / staging                        │
│                                                data-sync              18m fa           │
│           ┌─ Errore ──────────────────────────────────────────────────────────────┐   │
│           │ ErrImagePull: manifest for registry.example.com/worker:v0.9.4         │   │
│           │ not found                                                              │   │
│           └────────────────────────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Pannello completo con tutti e 3 gli stati visibili insieme

```
┌───────────────────────────────────────────────────────────────────────────────────────┐
│  ◈ Attività live                                          Ultimi 10 eventi            │
│ ─────────────────────────────────────────────────────────────────────────────────── │
│                                                                                       │
│  ◉ [SB] Sara B. ha rilasciato v1.15.0 → api-gateway / prod   platform-api   adesso  │
│   ↑ VERDE PULSANTE (in_progress)                                                      │
│                                                                                       │
│  ● [MR] Marco R. ha rilasciato v3.2.1 → frontend / prod       customer-app   2m fa   │
│   ↑ VERDE SOLIDO (success)                                                            │
│                                                                                       │
│  ● [AR] Andrea R. ha rilasciato v2.0.0 → worker / integration  data-sync     5m fa   │
│   ↑ VERDE SOLIDO (success)                                                            │
│                                                                                       │
│  ● [LC] Laura C. ha rilasciato v0.9.4 → worker / staging       data-sync    18m fa   │
│   ↑ ROSSO (failure)                                                                   │
│   └─ ErrImagePull: manifest not found ──────────────────────────────────────────── ─ │
│                                                                                       │
│  ● [SB] Sara B. ha rilasciato v1.14.9 → api-gateway / prod     platform-api  1h fa   │
│   ↑ VERDE SOLIDO (success)                                                            │
│                                                                                       │
│  ● [GT] Giorgio T. ha rilasciato v0.3.1 → scheduler / dev     nexus-gateway  3h fa   │
│   ↑ VERDE SOLIDO (success)                                                            │
│                                                                                       │
│  ... (fino a 10 righe)                                                                │
│                                                                                       │
│  ──────────────────────────────────────────────────────────────────────────────────  │
│  Aggiornato automaticamente · Solo admin · GET /api/v1/admin/activity                 │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Legenda colori (sistema CSS esistente)

| Stato         | Classe CSS                  | Colore         | Comportamento              |
|---------------|-----------------------------|----------------|----------------------------|
| `in_progress` | `.dot-pulse` (nuova)        | `#00c97a`      | pulse animation, ring glow |
| `success`     | `.dot-ok` (esistente)       | `#00c97a`      | glow statico               |
| `failure`     | `.dot-err` (esistente)      | `#e05555`      | glow statico               |

**Avatar iniziali:** cerchi colorati con sfondo gradiente (pattern esistente da `AdminPage.tsx`)  
**Tag versione:** stile `deploy-tag` con font mono e sfondo `rgba(144,208,200,.08)`  
**Slug prodotto:** `font-family: JetBrains Mono`, colore `--text-muted`  
**Timestamp relativo:** colore `--text-secondary`, aggiornato client-side  

---

## Note di implementazione per lo sviluppatore

- Il pannello si aggancia **sotto** la `.table-section` esistente nel layout di `AdminPage.tsx`.
- Nessuna colonna aggiuntiva: il pannello occupa il 100% della larghezza del content area.
- La riga `in_progress` usa `@keyframes pulse` + `box-shadow` animato (nuovo keyframe da aggiungere ad `AdminPage.css`).
- Il messaggio di errore (`error_message`) appare solo per `outcome === 'failure'`, in un `<div>` collassabile sotto la riga.
- Il pannello non ha paginazione: mostra sempre e solo i 10 record più recenti.
- Polling opzionale ogni 30s (o WebSocket in futuro) per l'effetto "live".
- Accesso: solo ruolo `DevOps Admin` (RBAC già gestito dal middleware esistente).
