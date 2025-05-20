# KeylightD Documentation

This directory contains the documentation for KeylightD, a daemon service for controlling Elgato Key Light devices.

## Directory Structure

- `docs/` - Markdown documentation files
  - `api/` - API reference documentation
  - `stylesheets/` - Custom CSS styles
- `mkdocs.yml` - MkDocs configuration
- `openapi.yaml` - OpenAPI 3.1.0 specification
- `mkdocs-serve.sh` - Script to run MkDocs locally with Docker/Podman
- `mkdocs-build.sh` - Script to build MkDocs site with Docker/Podman

**Note**: The directory structure follows MkDocs conventions:
- The Markdown source files live in `docs/docs/`
- The generated site will be output to `docs/site/` when building locally
- When deployed, the site will be published to the `gh-pages` branch

## Building the Documentation

The documentation is built using [MkDocs](https://www.mkdocs.org/) with the [Material theme](https://squidfunk.github.io/mkdocs-material/).

### Local Development with Docker/Podman

For convenience, scripts are provided to run MkDocs using Docker/Podman without installing any dependencies locally.

#### Prerequisites

- Docker or Podman installed on your system
- Git (for deployment)

#### Development Server

To run a local development server:

```bash
./mkdocs-serve.sh
```

This will start a server at http://localhost:8000 where you can preview the documentation.
The documentation will auto-reload as you edit the source files.

You can customize the port by setting the PORT environment variable:

```bash
PORT=8080 ./mkdocs-serve.sh
```

#### Building Static Site

To build the static site:

```bash
./mkdocs-build.sh
```

This will generate the static site in the `site` directory.

### Manual Installation (Alternative)

If you prefer to install MkDocs locally:

```bash
pip install mkdocs-material mkdocs-swagger-ui-tag
cd docs
mkdocs serve  # For development server
# or
mkdocs build  # For building static site
```

## Deployment

The documentation is automatically deployed to GitHub Pages when changes are pushed to the main branch.

## Contributing

To contribute to the documentation:

1. Make your changes to the Markdown files in the `docs/docs/` directory
2. Update the OpenAPI specification (`openapi.yaml`) if modifying API documentation
3. Run the development server to preview your changes (`./mkdocs-serve.sh`)
4. Ensure all links work and the content displays correctly
5. Submit a pull request with your changes

## API Documentation

The API documentation is generated from the OpenAPI specification (`openapi.yaml`). When updating the API, be sure to update the specification accordingly.

## Troubleshooting

- If images aren't displaying, ensure they're in the `docs/docs/assets/` directory and referenced using relative paths
- If the OpenAPI specification isn't loading, check for syntax errors in `openapi.yaml`
- For Docker/Podman permission issues, ensure the script is executable (`chmod +x mkdocs-*.sh`)