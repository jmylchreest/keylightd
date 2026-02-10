import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';
import apiSidebar from './docs/api/rest/sidebar';

const sidebars: SidebarsConfig = {
  docs: [
    'intro',
    'getting-started',
    {
      type: 'category',
      label: 'Supported Devices',
      link: {
        type: 'doc',
        id: 'supported-devices/index',
      },
      items: [
        'supported-devices/elgato',
      ],
    },
    {
      type: 'category',
      label: 'Lights',
      items: [
        'lights/cli',
        'lights/http',
        'lights/socket',
      ],
    },
    {
      type: 'category',
      label: 'Groups',
      items: [
        'groups/cli',
        'groups/http',
        'groups/socket',
      ],
    },
    {
      type: 'category',
      label: 'Desktop Apps',
      items: [
        'desktop-apps/tray',
        'desktop-apps/gnome-extension',
        'desktop-apps/waybar',
      ],
    },
    {
      type: 'category',
      label: 'API Reference',
      items: [
        {
          type: 'category',
          label: 'HTTP REST API',
          link: {
            type: 'doc',
            id: 'api/rest/keylightd-api',
          },
          items: apiSidebar as any[],
        },
        'api/unix-socket',
      ],
    },
  ],
};

export default sidebars;
