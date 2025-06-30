# Old Faithful

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.0-blue)](https://www.typescriptlang.org/)
[![Next.js](https://img.shields.io/badge/Next.js-15-black)](https://nextjs.org/)

Old Faithful (OF) is a comprehensive framework built on top of Triton's Old Faithful system for archival access to Solana blockchain data. It provides a modern web interface and robust backend for managing, indexing, and serving historical Solana epoch data from multiple data sources.

## Features

- **Multi-Source Data Management**: Support for S3, HTTP, and filesystem data sources
- **Epoch Management**: Track and manage Solana blockchain epochs with status monitoring
- **Background Processing**: Asynchronous job processing with Faktory for efficient data indexing
- **Index Management**: Support for multiple index types (CidToOffsetAndSize, SigExists, etc.)
- **GSFA Indexing**: GetSignaturesForAddress (GSFA) index support for efficient address lookups
- **Modern UI**: Clean, responsive interface built with Next.js 15 and shadcn/ui
- **API-First Design**: RESTful API endpoints for programmatic access
- **Real-time Updates**: Live status updates using TanStack Query

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [Architecture](#architecture)
- [API Documentation](#api-documentation)
- [Development](#development)
- [Testing](#testing)
- [Deployment](#deployment)
- [Contributing](#contributing)
- [License](#license)

## Prerequisites

- Node.js 20.x or higher
- PostgreSQL 14+ database
- Faktory (for background job processing)
- pnpm (recommended) or npm
- Docker (optional, for containerized deployment)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/notwedtm/old-faithful.git
cd old-faithful
```

2. Install dependencies:
```bash
pnpm install
```

3. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. Set up the database:
```bash
pnpm prisma migrate dev
pnpm seed
```

5. Start the development server:
```bash
# In one terminal, start Faktory
faktory

# In another terminal, start the worker
pnpm worker

# In another terminal, start the web server
pnpm dev
```

The application will be available at http://localhost:3000

## Configuration

### Environment Variables

See `.env.example` for all available configuration options. Key variables include:

- **Database**: `DATABASE_URL` - PostgreSQL connection string
- **AWS/S3**: AWS credentials for S3 data sources
- **HTTP Index**: Credentials for HTTP-based index services
- **Faktory**: `FAKTORY_URL` - Connection to job queue server

### Data Sources

Configure data sources in your environment to connect to different epoch data providers:

```bash
# S3 Sources
AWS_ENDPOINT=s3.region.provider.com
AWS_ACCESS_KEY_ID=your_key
AWS_SECRET_ACCESS_KEY=your_secret

# HTTP Sources
INDEX_HOST=https://api.example.com
INDEX_USER=username
INDEX_PASS=password
```

## Usage

### Web Interface

Navigate to http://localhost:3000 to access the web interface where you can:

- View all epochs and their status
- Trigger epoch refreshes
- Monitor background job progress
- Access detailed epoch information
- Manage indexes and GSFA data

### CLI Commands

```bash
# Run specific background tasks
pnpm task fetch_epoch_cids <epoch>
pnpm task get_standard_indexes <epoch>
pnpm task get_gsfa_index <epoch>
pnpm task refresh_epoch <epoch>
pnpm task refresh_source <source>

# Database management
pnpm prisma studio  # Open Prisma Studio GUI
```

### API Endpoints

The application exposes RESTful API endpoints:

- `GET /api/epochs` - List all epochs
- `GET /api/epochs/:id` - Get epoch details
- `POST /api/epochs/:id/refresh` - Trigger epoch refresh
- `GET /api/jobs` - List background jobs
- `GET /api/stats` - Get system statistics

See [API Documentation](#api-documentation) for complete details.

## Architecture

### System Overview

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Next.js App   │────▶│   API Routes    │────▶│   PostgreSQL    │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                │
                                ▼
                        ┌─────────────────┐
                        │    Faktory      │
                        │  Job Queue      │
                        └─────────────────┘
                                │
                                ▼
                        ┌─────────────────┐
                        │  Worker Process │
                        └─────────────────┘
                                │
                ┌───────────────┼───────────────┐
                ▼               ▼               ▼
        ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
        │  S3 Source  │ │ HTTP Source │ │  FS Source  │
        └─────────────┘ └─────────────┘ └─────────────┘
```

### Key Components

- **Frontend**: Next.js 15 with React 19, TypeScript, Tailwind CSS
- **Backend**: Next.js API routes with Prisma ORM
- **Database**: PostgreSQL for persistent storage
- **Job Queue**: Faktory for background processing
- **Workers**: Node.js processes for async tasks

## Development

### Project Structure

```
src/
├── app/              # Next.js app router pages
├── components/       # React components
├── lib/             # Core business logic
│   ├── epochs/      # Epoch management
│   ├── tasks/       # Background job definitions
│   └── data-sources/# Source implementations
└── generated/       # Generated Prisma client
```

### Code Style

- TypeScript with strict mode enabled
- ESLint for code linting
- Prettier for code formatting (coming soon)

### Adding New Features

1. Create new components in `src/components/`
2. Add API routes in `src/app/api/`
3. Define background tasks in `src/lib/tasks/`
4. Update database schema in `prisma/schema.prisma`

## Testing

(Testing infrastructure coming soon)

```bash
# Run all tests
pnpm test

# Run unit tests
pnpm test:unit

# Run integration tests
pnpm test:integration

# Generate coverage report
pnpm test:coverage
```

## Deployment

### Docker

```bash
# Build the Docker image
docker build -t old-reliable .

# Run with Docker Compose
docker-compose up
```

### Kubernetes

Helm charts are provided in `deploy/chart/`:

```bash
helm install old-reliable ./deploy/chart \
  --values your-values.yaml
```

See [SECURITY.md](SECURITY.md) for secure deployment practices.

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details on:

- Code of conduct
- Development workflow
- Submitting pull requests
- Reporting issues

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built on top of [Old Faithful](https://github.com/rpcpool/triton) by Triton/RPC Pool
- Uses [Faktory](https://github.com/contribsys/faktory) for job processing
- UI components from [shadcn/ui](https://ui.shadcn.com/)

