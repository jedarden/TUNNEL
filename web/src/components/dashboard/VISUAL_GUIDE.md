# Dashboard Visual Guide

## Dashboard Layout

```
┌─────────────────────────────────────────────────────────────────────┐
│ Dashboard                                                           │
│ Monitor and manage your tunnel connections                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐              │
│ │ 📊 Total │ │ 👥 Active│ │ ⚡ Avg   │ │ 📈 Total │              │
│ │ Providers│ │ Connects │ │ Latency  │ │ Requests │              │
│ │    3     │ │    2  ↑  │ │ 102ms ↓  │ │ 6,912 ↑  │              │
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘              │
│                                                                     │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  Connections (3 total)                    Quick Actions            │
│                                            ┌────────────────────┐  │
│  ┌────────────┐ ┌────────────┐           │ ▶ Connect All      │  │
│  │ 🚇 Ngrok   │ │ ☁️  Cloudfl │           │ ⏹ Disconnect All   │  │
│  │ Connected  │ │ Connected  │           │ 📊 Run Diagnostics  │  │
│  │ Port 3000  │ │ Port 8080  │           │ ⚙️  Settings        │  │
│  │            │ │            │           └────────────────────┘  │
│  │ 🌐 abc.ngrk│ │ 🌐 tunnel. │                                   │
│  │ ⚡ 85ms     │ │ ⚡ 120ms    │           Recent Activity         │
│  │            │ │            │           ┌────────────────────┐  │
│  │ ▶ Disc ⚙️  │ │ ▶ Disc ⚙️  │           │ ✅ Ngrok connected │  │
│  └────────────┘ └────────────┘           │    5 mins ago      │  │
│                                            │                    │  │
│  ┌────────────┐                           │ ℹ️  Cloudflare     │  │
│  │ 💻 Localho │                           │    reconnected     │  │
│  │ Disconncted│                           │    10 mins ago     │  │
│  │ Port 4000  │                           │                    │  │
│  │            │                           │ ❌ Connection      │  │
│  │ 🌐 localh: │                           │    timeout         │  │
│  │            │                           │    15 mins ago     │  │
│  │ ▶ Conn ⚙️  │                           └────────────────────┘  │
│  └────────────┘                                                   │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

## Component Details

### 1. StatsCard

```
┌────────────────────────┐
│ 📊  Total Providers    │  ← Icon + Label
│     Configured         │  ← Description
│                        │
│     3           ↑12%   │  ← Value + Trend
└────────────────────────┘
```

**Variants:**
- Primary (blue): Main metrics
- Success (green): Positive indicators
- Warning (yellow): Attention needed
- Error (red): Issues
- Default (gray): Neutral stats

### 2. ConnectionCard (Collapsed)

```
┌──────────────────────────────────┐
│ 🚇  Ngrok          [Connected]   │  ← Icon + Provider + Status
│     Port 3000                    │  ← Port info
│                                  │
│ 🌐 https://abc123.ngrok.io      │  ← Public URL
│ ⚡ 85ms  latency                 │  ← Latency with color
│                                  │
│ [⏹ Disconnect]  [⚙️]             │  ← Action buttons
└──────────────────────────────────┘
```

### 3. ConnectionCard (Expanded - on click)

```
┌──────────────────────────────────┐
│ 🚇  Ngrok          [Connected]   │
│     Port 3000                    │
│                                  │
│ 🌐 https://abc123.ngrok.io      │
│ ⚡ 85ms  latency                 │
│ ─────────────────────────────── │  ← Divider
│ Protocol         Started         │
│ HTTPS            1 hour ago      │
│                                  │
│ Requests         Error Rate      │
│ 1,234            2.0%            │
│                                  │
│ [⏹ Disconnect]  [⚙️]             │
└──────────────────────────────────┘
```

### 4. ConnectionCard (Error State)

```
┌──────────────────────────────────┐
│ 💻  Localhost        [Error]     │
│     Port 4000                    │
│                                  │
│ 🌐 http://localhost:4000        │
│ ⚠️  Failed to connect to        │
│     localhost:4000 - service     │
│     not responding               │
│                                  │
│ [▶ Connect]  [⚙️]                │
└──────────────────────────────────┘
```

### 5. QuickActions Panel

```
┌────────────────────────┐
│ Quick Actions          │  ← Title
├────────────────────────┤
│ [▶ Connect All     ]  │
│ [⏹ Disconnect All  ]  │
│ [📊 Run Diagnostics ]  │
│ [⚙️  Settings       ]  │
└────────────────────────┘
```

### 6. ActivityFeed (Empty)

```
┌────────────────────────┐
│ Recent Activity        │
├────────────────────────┤
│         ⏰             │
│                        │
│  No recent activity    │
│                        │
└────────────────────────┘
```

### 7. ActivityFeed (With Events)

```
┌─────────────────────────────┐
│ Recent Activity             │
├─────────────────────────────┤
│ ✅ Ngrok tunnel connected   │
│ │  Successfully established │
│ │  on port 3000             │
│ │  5 minutes ago            │
│ │                           │
│ ℹ️  Cloudflare reconnected  │
│ │  Tunnel automatically     │
│ │  recovered...             │
│ │  10 minutes ago           │
│ │                           │
│ ❌ Connection timeout       │
│    Failed to connect to     │
│    localhost:4000...        │
│    15 minutes ago           │
└─────────────────────────────┘
```

## Status Badge Colors

```
[Connected]     → Green background, green text
[Connecting]    → Yellow background, yellow text
[Disconnected]  → Gray background, gray text
[Error]         → Red background, red text
```

## Latency Color Coding

```
⚡ 85ms   → Green  (< 100ms)   Good
⚡ 250ms  → Yellow (< 300ms)   Fair
⚡ 450ms  → Red    (≥ 300ms)   Poor
```

## Button Variants

```
Primary:     [Blue background]     Main actions
Secondary:   [Gray background]     Alternative actions
Ghost:       [Transparent]         Subtle actions
Danger:      [Red background]      Destructive actions
```

## Responsive Behavior

### Mobile (< 640px)
```
┌─────────────┐
│ Stats       │  1 column
│ [Stat 1]    │
│ [Stat 2]    │
│ [Stat 3]    │
│ [Stat 4]    │
│             │
│ Connections │  1 column
│ [Conn 1]    │
│ [Conn 2]    │
│             │
│ Quick Actns │  Full width
│ Activity    │
└─────────────┘
```

### Tablet (640px - 1023px)
```
┌───────────────────────┐
│ Stats     Stats       │  2 columns
│ [Stat 1]  [Stat 2]    │
│ [Stat 3]  [Stat 4]    │
│                       │
│ Connections           │  2 columns
│ [Conn 1]  [Conn 2]    │
│ [Conn 3]              │
│                       │
│ Quick Actions         │  Full width
│ Activity              │
└───────────────────────┘
```

### Desktop (≥ 1024px)
```
┌─────────────────────────────────────┐
│ Stats   Stats   Stats   Stats       │  4 columns
│ [Stat1] [Stat2] [Stat3] [Stat4]     │
│                                     │
│ Connections          │ Quick Actns  │  2:1 ratio
│ [Conn1] [Conn2]      │ Activity     │
│ [Conn3]              │              │
└─────────────────────────────────────┘
```

## Dark Mode Comparison

### Light Mode
```
Background:     White (#FFFFFF)
Text:           Gray-900 (#111827)
Borders:        Gray-200 (#E5E7EB)
Cards:          White with shadows
```

### Dark Mode
```
Background:     Gray-900 (#111827)
Text:           Gray-100 (#F3F4F6)
Borders:        Gray-800 (#1F2937)
Cards:          Gray-900 with subtle borders
```

## Interactive States

### Hover States
- Cards: Elevated shadow
- Buttons: Darker/lighter background
- Links: Underline

### Click States
- ConnectionCard: Expand/collapse animation
- Buttons: Scale slightly down (0.98)

### Loading States
- Buttons: Spinner icon + disabled
- Stats: Skeleton or previous value

### Focus States
- All interactive elements: Ring outline
- Color matches variant (blue for primary, etc.)
