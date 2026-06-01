# LLMWatch Frontend

A stunning dark-mode AI observability dashboard built with React 18, TypeScript, Vite, Tailwind CSS, and Recharts.

## Features

- 🔴 **Live SSE stream** — real-time call events via Server-Sent Events with auto-reconnect
- 🔄 **Auto-polling** — all metrics refresh every 5 seconds
- 📊 **Rich charts** — calls/min area chart, latency percentiles, cost donut, error rate bars
- 🌐 **Provider comparison** — side-by-side provider performance with progress bars
- 📋 **Live call feed** — scrollable table of last 50 calls with highlight-on-arrival
- 🌙 **Dark glassmorphism design** — deep navy bg, indigo accents, glass cards

## Quick Start

```bash
# Install dependencies
npm install

# Start dev server (proxies /api and /events to backend at :8080)
npm run dev

# Build for production
npm run build
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `VITE_API_URL` | `http://localhost:8080` | Backend API base URL |

Create a `.env.local` file:
```
VITE_API_URL=http://your-backend:8080
```

## Docker

```bash
# Build image
docker build --build-arg VITE_API_URL=http://your-backend:8080 -t llmwatch-frontend .

# Run container
docker run -p 3000:80 llmwatch-frontend
```

## Project Structure

```
src/
├── types/
│   └── api.ts                  # TypeScript interfaces
├── hooks/
│   ├── useMetrics.ts           # 5s polling hook
│   └── useSSE.ts               # SSE connection hook
└── components/
    ├── Header.tsx              # Sticky header w/ live indicator
    ├── MetricCard.tsx          # Glassmorphism KPI card
    ├── StatusBadge.tsx         # Color-coded status pill
    ├── CallsPerMinuteChart.tsx # Area chart (60 min window)
    ├── LatencyPercentilesChart.tsx # P50/P95/P99 bar chart
    ├── CostByProviderChart.tsx # Interactive donut chart
    ├── ErrorRateChart.tsx      # Horizontal bar chart
    ├── LiveCallFeed.tsx        # Live call table
    ├── ProviderComparisonTable.tsx # Provider comparison cards
    └── Dashboard.tsx           # Main layout orchestrator
```

## API Endpoints Used

| Method | Endpoint | Description |
|---|---|---|
| GET | `/api/v1/metrics/summary` | KPI summary |
| GET | `/api/v1/metrics/calls-per-minute` | 60-point time series |
| GET | `/api/v1/metrics/latency-percentiles` | P50/P95/P99 by model |
| GET | `/api/v1/metrics/cost-by-provider` | Cost breakdown |
| GET | `/api/v1/metrics/error-rates` | Error rates by model |
| GET | `/api/v1/calls` | Recent calls list |
| GET | `/api/v1/metrics/provider-comparison` | Provider stats |
| GET | `/events/stream` | SSE live event stream |
