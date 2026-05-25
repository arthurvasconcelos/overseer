import { defineConfig } from "vitepress";

export default defineConfig({
  title: "overseer",
  description: "A personal machine management CLI for developers.",
  lang: "en-US",
  base: process.env.DOCS_BASE ?? "/overseer/",

  head: [
    ["link", { rel: "icon", href: "/overseer/favicon.svg", type: "image/svg+xml" }],
  ],

  themeConfig: {
    nav: [
      { text: "Install", link: "/install" },
      { text: "Commands", link: "/commands/", activeMatch: "/commands/" },
      { text: "Concepts", link: "/concepts/", activeMatch: "/concepts/" },
      { text: "Plugins", link: "/plugins/", activeMatch: "/plugins/" },
      {
        text: "More",
        items: [
          {
            text: "Changelog",
            link: "https://github.com/arthurvasconcelos/overseer/blob/main/CHANGELOG.md",
          },
          {
            text: "Contributing",
            link: "https://github.com/arthurvasconcelos/overseer/blob/main/CONTRIBUTING.md",
          },
          {
            text: "Releases",
            link: "https://github.com/arthurvasconcelos/overseer/releases",
          },
        ],
      },
    ],

    sidebar: {
      "/commands/": [
        { text: "Overview", link: "/commands/" },
        {
          text: "Reference",
          items: [
            { text: "accounts", link: "/commands/accounts" },
            { text: "brain", link: "/commands/brain" },
            { text: "brew", link: "/commands/brew" },
            { text: "completion", link: "/commands/completion" },
            { text: "config", link: "/commands/config" },
            { text: "context", link: "/commands/context" },
            { text: "daily", link: "/commands/daily" },
            { text: "env", link: "/commands/env" },
            { text: "focus", link: "/commands/focus" },
            { text: "git", link: "/commands/git" },
            { text: "init", link: "/commands/init" },
            { text: "mcp", link: "/commands/mcp" },
            { text: "note", link: "/commands/note" },
            { text: "notify", link: "/commands/notify" },
            { text: "plugins", link: "/commands/plugins" },
            { text: "prs", link: "/commands/prs" },
            { text: "repos", link: "/commands/repos" },
            { text: "run", link: "/commands/run" },
            { text: "setup", link: "/commands/setup" },
            { text: "ssh", link: "/commands/ssh" },
            { text: "standup", link: "/commands/standup" },
            { text: "status", link: "/commands/status" },
            { text: "update", link: "/commands/update" },
            { text: "weekly", link: "/commands/weekly" },
          ],
        },
      ],
      "/concepts/": [
        {
          text: "Concepts",
          items: [
            { text: "Overview", link: "/concepts/" },
            { text: "Brain", link: "/concepts/brain" },
            { text: "Config", link: "/concepts/config" },
            { text: "Secrets", link: "/concepts/secrets" },
          ],
        },
      ],
      "/plugins/": [
        {
          text: "Plugins",
          items: [
            { text: "Overview", link: "/plugins/" },
            { text: "Native plugins", link: "/plugins/native" },
            { text: "Python SDK", link: "/plugins/python" },
            { text: "TypeScript SDK", link: "/plugins/typescript" },
          ],
        },
      ],
    },

    socialLinks: [{ icon: "github", link: "https://github.com/arthurvasconcelos/overseer" }],

    footer: {
      message: 'Released under the <a href="https://opensource.org/licenses/MIT" target="_blank">MIT License</a>.',
      copyright: `Copyright © ${new Date().getFullYear()} Arthur Vasconcelos`,
    },

    search: {
      provider: "local",
    },

    editLink: {
      pattern: "https://github.com/arthurvasconcelos/overseer/edit/main/docs/:path",
      text: "Edit this page on GitHub",
    },
  },
});
