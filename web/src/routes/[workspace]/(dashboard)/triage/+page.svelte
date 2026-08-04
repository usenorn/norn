<script lang="ts">
	import { invalidateAll } from "$app/navigation";
	import { page } from "$app/state";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Zap from "@lucide/svelte/icons/zap";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import IssueRow from "$lib/components/norn/issue-row.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import {
		readTriageFailure,
		sourceLabels,
		triageFailureMessage,
		type TriageFailure,
		type TriageListing,
	} from "$lib/triage/triage";
	import type { Issue } from "$lib/issues/issues";
	import { workspacePath } from "$lib/workspace/navigation";
	import { triagePreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? triagePreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let localFailure = $state<TriageFailure | null>(null);
	let working = $state("");
	let merging = $state<Issue | null>(null);
	let mergeInto = $state("");
	let announcement = $state("");

	const slug = $derived(data.workspace.slug);
	const listing = $derived<TriageListing>(preview?.listing ?? data.listing);
	const failure = $derived<TriageFailure | null>(preview?.failure ?? localFailure);
	const teams = $derived(preview?.teams ?? data.teams);
	const busy = $derived(working !== "");

	async function decide(issue: Issue, path: string, body?: Record<string, unknown>) {
		working = issue.id;
		localFailure = null;

		try {
			const { error } = await api.POST(
				`/workspaces/{workspaceId}/triage/{issueId}/${path}` as
					"/workspaces/{workspaceId}/triage/{issueId}/accept",
				{
					params: { path: { workspaceId: data.workspace.id, issueId: issue.id } },
					body: body as never,
				}
			);

			if (error) {
				localFailure = readTriageFailure(error);

				return;
			}

			announcement = `${issue.reference} was ${path === "reassign" ? "handed on" : `${path}ed`}.`;
			merging = null;
			mergeInto = "";
			await invalidateAll();
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			working = "";
		}
	}

	function startMerge(issue: Issue) {
		merging = merging?.id === issue.id ? null : issue;
		mergeInto = "";
		localFailure = null;
	}

	async function confirmMerge() {
		if (!merging || !mergeInto.trim()) return;

		const reference = mergeInto.trim().toUpperCase();
		const target = await api.GET("/workspaces/{workspaceId}/issues/by-reference/{reference}", {
			params: { path: { workspaceId: data.workspace.id, reference } },
		});

		if (!target.data) {
			localFailure = { kind: "unavailable" };

			return;
		}

		await decide(merging, "merge", { duplicateOfId: target.data.id });
	}
</script>

<svelte:head><title>Triage · {data.workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Zap class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Triage</h1>
			<span class="text-sm text-muted-foreground">Arrived, and nobody has decided yet</span>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div class="pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]">
			<p class="sr-only" role="status" aria-live="polite">{announcement}</p>

			{#if failure}
				<div class="px-4 pt-3">
					<Alert.Root variant="destructive">
						<CircleX aria-hidden="true" />
						<Alert.Title>That did not stick</Alert.Title>
						<Alert.Description>{triageFailureMessage(failure)}</Alert.Description>
					</Alert.Root>
				</div>
			{/if}

			{#if listing.kind === "loading"}
				<div class="m-4 h-40 animate-pulse rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if listing.kind === "unavailable"}
				<p class="p-6 text-md leading-normal text-muted-foreground">
					We could not load the triage queue. Nothing changed &mdash; wait a moment and try again.
				</p>
			{:else if listing.kind === "empty"}
				<div class="flex flex-col items-center gap-3 px-6 py-16 text-center">
					<p class="text-md font-medium tracking-snug text-ink-900">Nothing waiting</p>
					<p class="max-w-90 text-md leading-normal text-muted-foreground text-pretty">
						Anything filed by an integration, an agent, or someone outside the team lands here
						first, so the backlog only holds work somebody agreed to.
					</p>
					<Button href={workspacePath(slug, "/issues")} variant="secondary" size="sm">
						Go to issues
					</Button>
				</div>
			{:else}
				{#each listing.groups as group (group.team?.id ?? "unknown")}
					<section role="group" aria-label={group.team?.name ?? "Another team"}>
						<div
							class="sticky top-0 z-1 flex h-7.5 items-center gap-2 border-b border-line-default bg-background pr-3 pl-3.5"
						>
							<span class="font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase">
								{group.team?.name ?? "Another team"}
							</span>
							<span class="font-mono text-2xs text-muted-foreground tabular-nums">
								{group.waiting}
							</span>
							<span class="h-px flex-1 bg-line-default" aria-hidden="true"></span>
						</div>

						{#each group.issues as issue (issue.id)}
							<div class="border-b border-line-subtle">
								<div class="flex flex-wrap items-center gap-2 pr-3">
									<div class="min-w-0 flex-[1_1_240px]">
										<IssueRow
											{issue}
											href={workspacePath(slug, `/issues/${issue.reference}`)}
											now={data.now}
											timezone={data.workspace.timezone}
										/>
									</div>
									<span
										class="shrink-0 rounded-sm border border-line-default px-1.5 py-0.5 text-sm whitespace-nowrap text-muted-foreground"
									>
										{issue.triageSource ? sourceLabels[issue.triageSource] : "Unknown"}
									</span>
									<div class="flex shrink-0 items-center gap-1">
										<Button
											size="sm"
											disabled={busy}
											onclick={() => decide(issue, "accept")}
										>
											{working === issue.id ? "Working" : "Accept"}
										</Button>
										<Button
											variant="secondary"
											size="sm"
											disabled={busy}
											onclick={() => startMerge(issue)}
										>
											Merge
										</Button>
										<DropdownMenu.Root>
											<DropdownMenu.Trigger>
												{#snippet child({ props })}
													<Button {...props} variant="secondary" size="sm" disabled={busy}>
														Hand on
													</Button>
												{/snippet}
											</DropdownMenu.Trigger>
											<DropdownMenu.Content align="end">
												<DropdownMenu.Label>Give it to</DropdownMenu.Label>
												{#each teams.filter((team) => team.id !== issue.teamId) as team (team.id)}
													<DropdownMenu.Item
														onSelect={() =>
															decide(issue, "reassign", {
																teamId: team.id,
																expectedVersion: issue.version,
																acknowledgeLabelLoss: true,
															})}
													>
														{team.name}
													</DropdownMenu.Item>
												{/each}
											</DropdownMenu.Content>
										</DropdownMenu.Root>
										<Button
											variant="ghost"
											size="sm"
											disabled={busy}
											onclick={() => decide(issue, "decline")}
										>
											Decline
										</Button>
									</div>
								</div>

								{#if merging?.id === issue.id}
									<div class="flex flex-wrap items-end gap-2 bg-paper-2 px-3.5 py-3">
										<div class="flex min-w-0 flex-[1_1_200px] flex-col gap-1">
											<label for="merge-into" class="text-sm text-muted-foreground">
												Which issue already says this?
											</label>
											<Input
												id="merge-into"
												bind:value={mergeInto}
												disabled={busy}
												placeholder="MOB-14"
												class="h-7.5"
											/>
										</div>
										<Button size="sm" disabled={busy || !mergeInto.trim()} onclick={confirmMerge}>
											{busy ? "Merging" : "Merge as duplicate"}
										</Button>
										<Button
											variant="secondary"
											size="sm"
											disabled={busy}
											onclick={() => (merging = null)}
										>
											Cancel
										</Button>
									</div>
								{/if}
							</div>
						{/each}
					</section>
				{/each}
			{/if}
		</div>
	</div>
</div>
