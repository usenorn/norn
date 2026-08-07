<script lang="ts">
	import ExternalLink from "@lucide/svelte/icons/external-link";
	import GitBranch from "@lucide/svelte/icons/git-branch";
	import GitCommitHorizontal from "@lucide/svelte/icons/git-commit-horizontal";
	import GitPullRequest from "@lucide/svelte/icons/git-pull-request";

	import { Button } from "$lib/components/ui/button";
	import {
		changeStateLabel,
		checksLabel,
		linkKindLabel,
		linkTitle,
		providerLabel,
		reviewVerdictLabel,
		type CodeLink,
	} from "./source-control";

	let {
		links,
		onunlink,
		busy = false,
	}: { links: CodeLink[]; onunlink?: (link: CodeLink) => void; busy?: boolean } = $props();

	const icons = {
		branch: GitBranch,
		commit: GitCommitHorizontal,
		change: GitPullRequest,
	};

	const checkTones = {
		pending: "text-muted-foreground",
		passing: "text-success",
		failing: "text-destructive",
	};

	const tones: Record<CodeLink["state"], string> = {
		draft: "text-muted-foreground",
		open: "text-ink-900",
		review_requested: "text-ink-900",
		changes_requested: "text-warning",
		approved: "text-success",
		merged: "text-success",
		closed: "text-muted-foreground",
		reopened: "text-ink-900",
		conflicted: "text-destructive",
	};
</script>

{#if links.length === 0}
	<p class="text-sm text-muted-foreground">
		Nothing yet. Name an issue in a branch or a commit and it appears here.
	</p>
{:else}
	<ul class="flex flex-col gap-2">
		{#each links as link (link.id)}
			{@const Icon = icons[link.kind]}
			<li class="flex items-start gap-2 rounded-md border border-line-subtle px-3 py-2">
				<Icon class="mt-0.5 size-icon-row shrink-0 text-muted-foreground" aria-hidden="true" />

				<div class="flex min-w-0 flex-col gap-0.5">
					<a
						href={link.url}
						target="_blank"
						rel="noreferrer"
						class="flex items-center gap-1 text-sm font-medium break-words text-ink-900 hover:underline"
					>
						{linkTitle(link)}
						<ExternalLink class="size-3 shrink-0" aria-hidden="true" />
					</a>

					<p class="text-xs text-muted-foreground">
						{linkKindLabel(link.kind)} · {providerLabel(link.provider)} · {link.repository}
						{#if link.author}· {link.author}{/if}
						<span class={tones[link.state]}>· {changeStateLabel(link.state)}</span>
						{#if link.checks}
							<span class={checkTones[link.checks]}>· {checksLabel(link.checks)}</span>
						{/if}
					</p>

					{#if link.reviewers?.length}
						<ul class="flex flex-wrap gap-x-2 gap-y-0.5 text-xs text-muted-foreground">
							{#each link.reviewers as reviewer (reviewer.login)}
								<li class:text-destructive={reviewer.verdict === "changes_requested"}>
									{reviewer.login} — {reviewVerdictLabel(reviewer.verdict)}
								</li>
							{/each}
						</ul>
					{/if}

					{#if link.kind === "change" && link.resolving === false}
						<p class="text-xs text-muted-foreground">
							Mentions this issue. Write “fixes {"{"}reference{"}"}” in the change for it to move
							the issue on.
						</p>
					{/if}

					{#if !link.connected}
						<p class="text-xs text-muted-foreground">
							The connection that found this was removed. The link is kept.
						</p>
					{/if}
				</div>

				{#if onunlink}
					<Button
						variant="ghost"
						class="ml-auto shrink-0"
						disabled={busy}
						onclick={() => onunlink(link)}
					>
						Unlink
					</Button>
				{/if}
			</li>
		{/each}
	</ul>
{/if}
