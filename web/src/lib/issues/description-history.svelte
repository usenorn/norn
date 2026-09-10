<script lang="ts">
	import CircleX from "@lucide/svelte/icons/circle-x";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { api } from "$lib/api";
	import Editor from "$lib/editor/editor.svelte";
	import { asDocument } from "$lib/editor/document";
	import type { components } from "$lib/api/dashboard.gen";

	type Revision = components["schemas"]["DescriptionRevision"];

	type Trail =
		| { kind: "loading" }
		| { kind: "empty" }
		| { kind: "ready"; revisions: Revision[] }
		| { kind: "unavailable" };

	let {
		open = $bindable(false),
		workspaceId,
		workspace,
		issueId,
		version,
		when,
		nameOf,
		working = false,
		onrestored,
	}: {
		open?: boolean;
		workspaceId: string;
		workspace: string;
		issueId: string;
		version: number;
		when: (instant: string) => string;
		nameOf: (accountId: string) => string;
		working?: boolean;
		onrestored?: () => void;
	} = $props();

	let trail = $state.raw<Trail>({ kind: "loading" });
	let chosen = $state("");
	let restoring = $state(false);
	let failure = $state("");

	const sources: Record<string, string> = {
		person: "written by hand",
		agent: "written by an agent",
		import: "brought in by an import",
		intake: "arrived by email",
		restore: "put back from the history",
	};

	const revisions = $derived(trail.kind === "ready" ? trail.revisions : []);
	const shown = $derived(revisions.find((revision) => revision.id === chosen) ?? revisions[0]);

	$effect(() => {
		if (!open) return;

		void read();
	});

	async function read() {
		trail = { kind: "loading" };
		failure = "";

		try {
			const { data, error } = await api.GET(
				"/workspaces/{workspaceId}/issues/{issueId}/description/revisions",
				{ params: { path: { workspaceId, issueId } } }
			);

			if (error || !data) {
				trail = { kind: "unavailable" };

				return;
			}

			trail = data.revisions.length === 0 ? { kind: "empty" } : { kind: "ready", revisions: data.revisions };
			chosen = data.revisions[0]?.id ?? "";
		} catch {
			trail = { kind: "unavailable" };
		}
	}

	async function restore(revisionId: string) {
		restoring = true;
		failure = "";

		try {
			const { error } = await api.POST(
				"/workspaces/{workspaceId}/issues/{issueId}/description/revisions/{revisionId}/restore",
				{
					params: { path: { workspaceId, issueId, revisionId } },
					body: { expectedVersion: version },
				}
			);

			if (error) {
				failure =
					"The description moved on while this was open. Close the history and open it again.";

				return;
			}

			open = false;
			onrestored?.();
		} catch {
			failure = "We could not reach the server just now. Nothing was changed.";
		} finally {
			restoring = false;
		}
	}

	function attribution(revision: Revision): string {
		const author = revision.authorName || (revision.authorAccountId ? nameOf(revision.authorAccountId) : "");
		const said = sources[revision.source] ?? revision.source;

		return author ? `${author} · ${said}` : said;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="max-h-[calc(100dvh-6rem)] overflow-hidden sm:max-w-3xl">
		<Dialog.Header>
			<Dialog.Title>What this description has said</Dialog.Title>
			<Dialog.Description>
				Every version is kept. Putting one back writes it forward as a new version, so the text
				it replaces is kept too.
			</Dialog.Description>
		</Dialog.Header>

		{#if failure}
			<Alert.Root variant="destructive">
				<CircleX aria-hidden="true" />
				<Alert.Title>That did not work</Alert.Title>
				<Alert.Description>{failure}</Alert.Description>
			</Alert.Root>
		{/if}

		{#if trail.kind === "loading"}
			<div class="h-40 animate-breathe rounded-lg bg-paper-2" aria-busy="true"></div>
		{:else if trail.kind === "unavailable"}
			<p class="text-md text-muted-foreground">We could not read the history of this description.</p>
		{:else if trail.kind === "empty"}
			<p class="text-md text-muted-foreground">
				This description has not been written since versions were kept.
			</p>
		{:else}
			<div class="grid min-h-0 gap-3.5 sm:grid-cols-[14rem_minmax(0,1fr)]">
				<ul class="max-h-96 min-w-0 overflow-y-auto" aria-label="Versions">
					{#each revisions as revision (revision.id)}
						<li>
							<button
								type="button"
								aria-current={revision.id === shown?.id}
								onclick={() => (chosen = revision.id)}
								class="flex w-full cursor-pointer flex-col gap-0.5 rounded-md px-2 py-1.5 text-left motion-control hover:bg-accent {revision.id ===
								shown?.id
									? 'bg-accent'
									: ''}"
							>
								<span class="text-sm text-ink-900">
									<time datetime={revision.createdAt}>{when(revision.createdAt)}</time>
								</span>
								<span class="text-xs text-muted-foreground">{attribution(revision)}</span>
							</button>
						</li>
					{/each}
				</ul>

				<div class="flex min-w-0 flex-col gap-2">
					{#if shown}
						<Eyebrow class="text-ink-600">Version {shown.issueVersion}</Eyebrow>
						<div class="max-h-80 min-w-0 overflow-y-auto rounded-md border border-line-subtle p-2.5">
							<Editor
								document={asDocument(shown.doc)}
								{workspaceId}
								{workspace}
								disabled
								minHeight="min-h-0"
								label="What the description said"
							/>
						</div>
						<div>
							<Button
								size="sm"
								disabled={working || restoring}
								onclick={() => restore(shown.id)}
							>
								{restoring ? "Putting it back" : "Put this back"}
							</Button>
						</div>
					{/if}
				</div>
			</div>
		{/if}
	</Dialog.Content>
</Dialog.Root>
