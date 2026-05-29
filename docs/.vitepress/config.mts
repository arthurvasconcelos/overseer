import type { DefaultTheme, HeadConfig, TransformContext } from "vitepress";
import { defineConfig } from "vitepress";
import llmstxt, { copyOrDownloadAsMarkdownButtons } from "vitepress-plugin-llms";
import { groupIconMdPlugin, groupIconVitePlugin } from "vitepress-plugin-group-icons";

const base = process.env.DOCS_BASE ?? "/overseer/";
const siteUrl = process.env.SITE_URL ?? "https://arthurvasconcelos.github.io";

function sidebarCommands(): DefaultTheme.SidebarItem[] {
  return [
    { text: "Overview", link: "/commands/" },
    {
      text: "Reference",
      items: [
        { text: "accounts", link: "/commands/accounts" },
        { text: "brain", link: "/commands/brain" },
        { text: "brew", link: "/commands/brew" },
        { text: "claude", link: "/commands/claude" },
        { text: "completion", link: "/commands/completion" },
        { text: "config", link: "/commands/config" },
        { text: "context", link: "/commands/context" },
        { text: "daily", link: "/commands/daily" },
        { text: "env", link: "/commands/env" },
        { text: "focus", link: "/commands/focus" },
        { text: "git", link: "/commands/git" },
        { text: "init", link: "/commands/init" },
        { text: "learn", link: "/commands/learn" },
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
  ];
}

function sidebarConcepts(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: "Concepts",
      items: [
        { text: "Overview", link: "/concepts/" },
        { text: "Brain", link: "/concepts/brain" },
        { text: "Config", link: "/concepts/config" },
        { text: "Secrets", link: "/concepts/secrets" },
      ],
    },
  ];
}

function sidebarPlugins(): DefaultTheme.SidebarItem[] {
  return [
    {
      text: "Plugins",
      items: [
        { text: "Overview", link: "/plugins/" },
        { text: "Native plugins", link: "/plugins/native" },
        { text: "Python SDK", link: "/plugins/python" },
        { text: "TypeScript SDK", link: "/plugins/typescript" },
      ],
    },
  ];
}

export default defineConfig({
  title: "overseer",
  description: "A personal machine management CLI for developers.",
  lang: "en-US",
  base,
  cleanUrls: true,
  lastUpdated: true,

  vite: {
    plugins: [llmstxt(), groupIconVitePlugin()],
  },

  markdown: {
    config(md) {
      md.use(groupIconMdPlugin);
      md.use(copyOrDownloadAsMarkdownButtons);
    },
  },

  head: [
    ["link", { rel: "icon", href: `${base}favicon.svg`, type: "image/svg+xml" }],
  ],

  transformHead({ pageData, page }: TransformContext): HeadConfig[] {
    const head: HeadConfig[] = [];
    const pageUrl = page.replace(/\.md$/, "").replace(/index$/, "");
    const canonicalUrl = `${siteUrl}${base}${pageUrl}`;

    head.push(["link", { rel: "canonical", href: canonicalUrl }]);
    head.push(["meta", { property: "og:title", content: pageData.title }]);
    if (pageData.description) {
      head.push(["meta", { property: "og:description", content: pageData.description }]);
    }
    return head;
  },

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
      { text: "For LLMs", link: "/llms" },
    ],

    sidebar: {
      "/commands/": sidebarCommands(),
      "/concepts/": sidebarConcepts(),
      "/plugins/": sidebarPlugins(),
    },

    socialLinks: [{ icon: "github", link: "https://github.com/arthurvasconcelos/overseer" }],

    footer: {
      message: 'Released under the <a href="https://github.com/arthurvasconcelos/overseer/blob/main/LICENSE" target="_blank">MIT License</a>.',
      copyright: `Copyright © ${new Date().getFullYear()} <a href="https://github.com/arthurvasconcelos" target="_blank">Arthur Vasconcelos</a>`,
    },

    search: {
      provider: "local",
      options: {
        miniSearch: {
          searchOptions: {
            boostDocument(documentId: string) {
              if (documentId.includes("/concepts/")) return 2;
              if (documentId.includes("/commands/")) return 1.5;
              if (documentId.includes("/plugins/")) return 1.2;
              return 1;
            },
          },
        },
      },
    },

    editLink: {
      pattern: "https://github.com/arthurvasconcelos/overseer/edit/main/docs/:path",
      text: "Edit this page on GitHub",
    },
  },
});
