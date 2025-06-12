# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of Old Reliable
- Multi-source data management (S3, HTTP, filesystem)
- Epoch management with status tracking
- Background job processing with Faktory
- Index management for multiple index types
- GSFA (GetSignaturesForAddress) index support
- Modern web interface with Next.js 15
- RESTful API endpoints
- Real-time status updates
- Docker support with docker-compose
- Comprehensive test suite with Jest
- Security measures and pre-commit hooks

### Security
- Removed all hardcoded secrets
- Added environment variable configuration
- Implemented pre-commit security checks
- Added .env.example for configuration reference

## [0.1.0] - 2024-12-06

### Added
- Initial project setup
- Basic epoch tracking functionality
- PostgreSQL database integration
- Faktory job queue integration

[Unreleased]: https://github.com/your-org/old-reliable/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/your-org/old-reliable/releases/tag/v0.1.0