---
title: overseer
layout: hextra-home
---

{{< hextra/hero-container >}}
  {{< hextra/hero-headline >}}Your developer workflow,&nbsp;<br class="sm:block hidden" />one binary.{{< /hextra/hero-headline >}}
  {{< hextra/hero-subtitle >}}
    overseer is a single Go binary that unifies daily developer workflows: git identities, repo management, morning briefings, notes, PR monitoring, and Homebrew — all driven by a private config repo you own.
  {{< /hextra/hero-subtitle >}}
  {{< hextra/hero-button text="Get started" link="/install" >}}
  {{< hextra/hero-button text="Command reference" link="/commands" style="outline" >}}
{{< /hextra/hero-container >}}

{{< cards >}}
  {{< card link="/concepts/brain" title="Brain" icon="folder" subtitle="A private git repo as your portable config store — decoupled from the tool." >}}
  {{< card link="/concepts/config" title="Config" icon="adjustments" subtitle="Two-layer merge: shared brain config on top of machine-local overrides." >}}
  {{< card link="/concepts/secrets" title="Secrets" icon="lock-closed" subtitle="1Password references resolved at runtime. Nothing stored in plaintext." >}}
  {{< card link="/commands" title="Commands" icon="terminal" subtitle="Full reference for all overseer subcommands and flags." >}}
  {{< card link="/plugins" title="Plugins" icon="puzzle" subtitle="Drop any overseer-* binary in your brain and it becomes a subcommand." >}}
  {{< card link="/install" title="Install" icon="cloud-download" subtitle="Homebrew, manual install, and first-time setup." >}}
{{< /cards >}}
