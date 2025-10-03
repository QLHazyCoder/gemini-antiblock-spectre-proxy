# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2024-12-20

### Added
- Lightweight deployment guide emphasizing 1 core CPU + 512MB RAM capability
- Server region selection recommendations with geographical considerations
- Caddy and 3x-ui tool recommendations for enhanced server utilization
- Comprehensive CI/CD documentation in README
- Docker multi-architecture support details (amd64 and arm64)

### Improved
- README structure with clearer deployment sections
- Documentation for GitHub Container Registry usage

## [1.0.0] - 2024-12-15

### Initial Release

#### Core Features
- **Streaming Response Handling**: Full support for Server-Sent Events (SSE) streaming
- **Intelligent Retry Mechanism**: Automatic retry on stream interruptions (up to 100 consecutive retries)
- **Thought Content Filtering**: Filter model's thinking process after retries for cleaner output
- **Standardized Error Responses**: Google API standard-compliant error response format
- **CORS Support**: Complete Cross-Origin Resource Sharing support
- **Rate Limiting**: Configurable request rate limiting functionality
- **Detailed Logging**: Debug mode support with comprehensive operational logs

#### Infrastructure
- Multi-architecture Docker images (linux/amd64, linux/arm64)
- GitHub Actions CI/CD pipeline for automated builds
- Docker Compose support for easy deployment
- Health check endpoint for monitoring

#### Configuration
- 11 environment variables for flexible configuration
- SpectreProxy Worker integration support
- Multi-worker load balancing with automatic rotation
- Punctuation heuristic optimization for retry logic

#### Developer Tools
- Mock server for testing various scenarios
- Comprehensive test suite coverage
- Detailed API documentation

### Technical Details
- Built with Go 1.21+
- Uses gorilla/mux for HTTP routing
- Implements sophisticated SSE stream processing
- Context-aware retry logic with request rebuilding
- Configurable retry delays and maximum attempts

---

## Version History

### Release Notes

**v1.2.0**: Enhanced documentation and deployment guidance focusing on lightweight VPS deployments

**v1.0.0**: Initial production-ready release with full feature set including streaming, retry mechanisms, and SpectreProxy integration
