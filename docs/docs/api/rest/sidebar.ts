import type { SidebarsConfig } from "@docusaurus/plugin-content-docs";

const sidebar: SidebarsConfig = {
  apisidebar: [
    {
      type: "doc",
      id: "api/rest/keylightd-api",
    },
    {
      type: "category",
      label: "Health",
      link: {
        type: "doc",
        id: "api/rest/health",
      },
      items: [
        {
          type: "doc",
          id: "api/rest/health-check",
          label: "Health check",
          className: "api-method get",
        },
      ],
    },
    {
      type: "category",
      label: "Lights",
      link: {
        type: "doc",
        id: "api/rest/lights",
      },
      items: [
        {
          type: "doc",
          id: "api/rest/list-lights",
          label: "List all lights",
          className: "api-method get",
        },
        {
          type: "doc",
          id: "api/rest/get-light",
          label: "Get a light",
          className: "api-method get",
        },
        {
          type: "doc",
          id: "api/rest/set-light-state",
          label: "Set light state",
          className: "api-method post",
        },
      ],
    },
    {
      type: "category",
      label: "Groups",
      link: {
        type: "doc",
        id: "api/rest/groups",
      },
      items: [
        {
          type: "doc",
          id: "api/rest/list-groups",
          label: "List all groups",
          className: "api-method get",
        },
        {
          type: "doc",
          id: "api/rest/create-group",
          label: "Create a group",
          className: "api-method post",
        },
        {
          type: "doc",
          id: "api/rest/get-group",
          label: "Get a group",
          className: "api-method get",
        },
        {
          type: "doc",
          id: "api/rest/delete-group",
          label: "Delete a group",
          className: "api-method delete",
        },
        {
          type: "doc",
          id: "api/rest/set-group-lights",
          label: "Set group lights",
          className: "api-method put",
        },
        {
          type: "doc",
          id: "api/rest/set-group-state",
          label: "Set group state",
          className: "api-method put",
        },
      ],
    },
    {
      type: "category",
      label: "API Keys",
      link: {
        type: "doc",
        id: "api/rest/api-keys",
      },
      items: [
        {
          type: "doc",
          id: "api/rest/list-api-keys",
          label: "List API keys",
          className: "api-method get",
        },
        {
          type: "doc",
          id: "api/rest/create-api-key",
          label: "Create an API key",
          className: "api-method post",
        },
        {
          type: "doc",
          id: "api/rest/delete-api-key",
          label: "Delete an API key",
          className: "api-method delete",
        },
        {
          type: "doc",
          id: "api/rest/set-api-key-disabled",
          label: "Enable or disable an API key",
          className: "api-method put",
        },
      ],
    },
    {
      type: "category",
      label: "Logging",
      link: {
        type: "doc",
        id: "api/rest/logging",
      },
      items: [
        {
          type: "doc",
          id: "api/rest/list-log-filters",
          label: "List log filters and current level",
          className: "api-method get",
        },
        {
          type: "doc",
          id: "api/rest/set-log-filters",
          label: "Replace all log filters",
          className: "api-method put",
        },
        {
          type: "doc",
          id: "api/rest/set-log-level",
          label: "Set global log level",
          className: "api-method put",
        },
      ],
    },
    {
      type: "category",
      label: "Version",
      items: [
        {
          type: "doc",
          id: "api/rest/get-version",
          label: "Daemon version",
          className: "api-method get",
        },
      ],
    },
  ],
};

export default sidebar.apisidebar;
