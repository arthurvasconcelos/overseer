---
title: overseer
layout: hextra-home
---

<div class="hx:grid home-hero-columns hx:gap-4">
{{< hextra/hero-container cols="1" >}}
{{< hextra/hero-headline >}}Your developer workflow,&nbsp;<br class="sm:block hidden" />one binary.{{< /hextra/hero-headline >}}
{{< hextra/hero-subtitle >}}
Git identities, repos, Homebrew, daily briefings, notes, and PR monitoring — one binary, driven by a private config repo you own.
{{< /hextra/hero-subtitle >}}
<div class="hx:mt-6">
{{< hextra/hero-button text="Get started" link="/docs/install" >}}
{{< hextra/hero-button text="Command reference" link="/docs/commands" style="outline" >}}
</div>
{{< /hextra/hero-container >}}

{{< cards >}}
{{< card link="/docs/concepts/brain" title="Brain" icon="folder" subtitle="A private git repo as your portable config store — decoupled from the tool." >}}
{{< card link="/docs/concepts/config" title="Config" icon="adjustments" subtitle="Two-layer merge: shared brain config on top of machine-local overrides." >}}
{{< card link="/docs/concepts/secrets" title="Secrets" icon="lock-closed" subtitle="1Password references resolved at runtime. Nothing stored in plaintext." >}}
{{< card link="/docs/commands" title="Commands" icon="terminal" subtitle="Full reference for all overseer subcommands and flags." >}}
{{< card link="/docs/plugins" title="Plugins" icon="puzzle" subtitle="Drop any overseer-* binary in your brain and it becomes a subcommand." >}}
{{< card link="/docs/install" title="Install" icon="cloud-download" subtitle="Homebrew, manual install, and first-time setup." >}}
{{< /cards >}}
</div>