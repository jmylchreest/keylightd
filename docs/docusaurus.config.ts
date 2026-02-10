import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';
import type * as OpenApiPlugin from 'docusaurus-plugin-openapi-docs';

const config: Config = {
  title: 'keylightd',
  tagline: 'A daemon for discovering, monitoring, and controlling Elgato Key Light devices',
  favicon: 'img/logo.svg',

  // GitHub Pages deployment
  url: 'https://jmylchreest.github.io',
  baseUrl: '/keylightd/',
  organizationName: 'jmylchreest',
  projectName: 'keylightd',
  deploymentBranch: 'gh-pages',
  trailingSlash: false,

  onBrokenLinks: 'throw',

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/jmylchreest/keylightd/tree/main/docs/',
          includeCurrentVersion: true,
          versions: {
            current: {
              label: 'main',
              path: 'next',
              banner: 'unreleased',
            },
          },
          lastVersion: require('./versions.json')[0] || 'current',
          docItemComponent: '@theme/ApiItem',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  plugins: [
    // Local search
    [
      '@cmfcmf/docusaurus-search-local',
      {
        indexDocs: true,
        indexBlog: false,
        indexPages: true,
        language: 'en',
        maxSearchResults: 8,
      },
    ],
    // OpenAPI docs generation
    [
      'docusaurus-plugin-openapi-docs',
      {
        id: 'api-docs',
        docsPluginId: 'default',
        config: {
          keylightd: {
            specPath: 'static/openapi.yaml',
            outputDir: 'docs/api/rest',
            sidebarOptions: {
              groupPathsBy: 'tag',
              categoryLinkSource: 'tag',
            },
          } satisfies OpenApiPlugin.Options,
        },
      },
    ],
  ],

  themes: ['docusaurus-theme-openapi-docs'],

  themeConfig: {
    colorMode: {
      defaultMode: 'dark',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },

    navbar: {
      title: 'keylightd',
      logo: {
        alt: 'keylightd Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docs',
          position: 'left',
          label: 'Documentation',
        },
        {
          type: 'docsVersionDropdown',
          position: 'right',
          dropdownActiveClassDisabled: true,
        },
        {
          href: 'https://github.com/jmylchreest/keylightd',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },

    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {label: 'Getting Started', to: '/docs/getting-started'},
            {label: 'Lights', to: '/docs/lights/cli'},
            {label: 'Groups', to: '/docs/groups/cli'},
            {label: 'API Reference', to: '/docs/api/rest/keylightd-api'},
          ],
        },
        {
          title: 'Desktop Apps',
          items: [
            {label: 'Tray Application', to: '/docs/desktop-apps/tray'},
            {label: 'GNOME Extension', to: '/docs/desktop-apps/gnome-extension'},
            {label: 'Waybar', to: '/docs/desktop-apps/waybar'},
          ],
        },
        {
          title: 'Community',
          items: [
            {label: 'GitHub', href: 'https://github.com/jmylchreest/keylightd'},
            {label: 'Issues', href: 'https://github.com/jmylchreest/keylightd/issues'},
            {label: 'GNOME Extensions', href: 'https://extensions.gnome.org/extension/8185/keylightd-control/'},
          ],
        },
      ],
      copyright: `Copyright \u00a9 ${new Date().getFullYear()} keylightd. Built with Docusaurus.`,
    },

    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'json', 'yaml', 'python', 'css', 'go'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
