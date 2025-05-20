# Assets Directory

This directory contains images, diagrams, and other assets used in the KeylightD documentation.

## Guidelines

- Use descriptive filenames that reflect the content
- Optimize images for web (compress PNGs, use appropriate resolution)
- Use SVG for diagrams where possible
- Group assets in subdirectories for better organization:
  - `/screenshots/` - UI screenshots
  - `/diagrams/` - Architecture and flow diagrams
  - `/icons/` - Icon assets

## Usage

When referencing assets in Markdown, use relative paths:

```md
![API Architecture](../assets/diagrams/api-architecture.svg)
```

This ensures proper rendering both locally and when deployed.