<script lang="ts">
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import Download from "@lucide/svelte/icons/download";
	import ScrollText from "@lucide/svelte/icons/scroll-text";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { api } from "$lib/api";
	import {
		actionLabel,
		actorLabel,
		auditActions,
		authLabel,
		detailPairs,
		filtersApplied,
		noAuditFilters,
		resourceKinds,
		resourceLabel,
		type AuditEvent,
		type AuditFilters,
		type AuditRecord,
	} from "$lib/audit/audit";
	import { onDateAndTime } from "$lib/time";
	import type { PageProps } from "./$types";
	import { auditPreviewStates } from "./preview";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? auditPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let loaded = $state<AuditRecord | null>(null);
	let filters = $state<AuditFilters>({ ...noAuditFilters });
	let working = $state(false);
	let exporting = $state(false);
	let exportFailed = $state(false);

	const workspace = $derived(page.data.workspace);
	const record = $derived<AuditRecord>(loaded ?? preview?.record ?? data.record);
	const busy = $derived(preview?.busy ?? working);
	const shown = $derived(preview?.filters ?? filters);
	const retention = $derived(
		record.kind === "ready" || record.kind === "empty" || record.kind === "unlicensed"
			? record.retentionDays
			: 0
	);
	const events = $derived(record.kind === "ready" ? record.events : []);
	const readable = $derived(record.kind === "ready" || record.kind === "empty");

	function query(cursor?: string) {
		const applied = preview?.filters ?? filters;

		return {
			limit: 50,
			...(cursor ? { cursor } : {}),
			...(applied.action ? { action: [applied.action] } : {}),
			...(applied.actor ? { actor: applied.actor } : {}),
			...(applied.resourceKind ? { resourceKind: applied.resourceKind } : {}),
			...(applied.from ? { from: new Date(`${applied.from}T00:00:00Z`).toISOString() } : {}),
			...(applied.to ? { to: new Date(`${applied.to}T00:00:00Z`).toISOString() } : {}),
		};
	}

	async function read(cursor?: string) {
		working = true;

		const { data: found, error, response } = await api.GET("/workspaces/{workspaceId}/audit", {
			params: { path: { workspaceId: workspace.id }, query: query(cursor) },
		});

		working = false;

		if (error) {
			if (response.status === 503) {
				loaded = { kind: "unlicensed", retentionDays: retention };
			} else if (response.status === 403) {
				loaded = { kind: "forbidden" };
			} else {
				loaded = { kind: "unavailable" };
			}

			return;
		}

		if (!found) return;

		const carried = cursor && record.kind === "ready" ? record.events : [];
		const combined = [...carried, ...found.events];

		loaded =
			combined.length === 0
				? { kind: "empty", retentionDays: found.retentionDays }
				: {
						kind: "ready",
						events: combined,
						nextCursor: found.nextCursor,
						retentionDays: found.retentionDays,
					};
	}

	function clearFilters() {
		filters = { ...noAuditFilters };
		void read();
	}

	function exportPath(): string {
		const applied = preview?.filters ?? filters;
		const params = new URLSearchParams();

		if (applied.action) params.set("action", applied.action);
		if (applied.actor) params.set("actor", applied.actor);
		if (applied.resourceKind) params.set("resourceKind", applied.resourceKind);
		if (applied.from) params.set("from", new Date(`${applied.from}T00:00:00Z`).toISOString());
		if (applied.to) params.set("to", new Date(`${applied.to}T00:00:00Z`).toISOString());

		const search = params.toString();

		return `/v1/workspaces/${workspace.id}/audit/export${search ? `?${search}` : ""}`;
	}

	async function exportRecord() {
		exporting = true;
		exportFailed = false;

		try {
			const response = await fetch(exportPath(), { credentials: "same-origin" });

			if (!response.ok) {
				exportFailed = true;

				return;
			}

			const blob = await response.blob();
			const url = URL.createObjectURL(blob);
			const link = document.createElement("a");

			link.href = url;
			link.download = "norn-audit.ndjson";
			link.click();

			URL.revokeObjectURL(url);
		} catch {
			exportFailed = true;
		} finally {
			exporting = false;
		}
	}

	function occurred(event: AuditEvent): string {
		return onDateAndTime(event.occurredAt, workspace.timezone);
	}
</script>

<svelte:head><title>Audit log · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center justify-between gap-3 border-b border-line-subtle px-4">
		<h1 class="text-sm font-medium tracking-snug text-ink-900">Audit log</h1>
		{#if readable}
			<Button
				variant="ghost"
				size="sm"
				disabled={exporting || busy}
				onclick={exportRecord}
			>
				<Download aria-hidden="true" />
				{exporting ? "Exporting…" : "Export"}
			</Button>
		{/if}
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-240 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if exportFailed}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>The export did not finish</Alert.Title>
					<Alert.Description>Check your connection and try again.</Alert.Description>
				</Alert.Root>
			{/if}

			{#if record.kind === "unlicensed"}
				<section class="flex flex-col gap-3 rounded-lg border border-line-default bg-paper-0 p-5">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							The audit log is not part of this instance's licence
						</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Norn is still recording what happens. Reading the record needs a licence that
							covers the audit log.
						</p>
					</div>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Nothing recorded so far is lost, and nothing is being removed while this is off.
						<a class="underline underline-offset-2 hover:text-ink-900" href="/settings/licence">
							See what this instance is licensed for
						</a>.
					</p>
				</section>
			{:else if record.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not read the audit log</Alert.Title>
					<Alert.Description>
						Audit access is granted per person and is separate from administering {workspace.name}.
						Ask an administrator to grant it on the members screen.
					</Alert.Description>
				</Alert.Root>
			{:else if record.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load the audit log</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else}
				<section class="flex flex-col gap-3">
					<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
						<div class="flex flex-col gap-1.5">
							<Label for="audit-action">Action</Label>
							<Select.Root
								type="single"
								bind:value={filters.action}
								onValueChange={() => read()}
								disabled={busy}
							>
								<Select.Trigger id="audit-action">
									{filters.action ? actionLabel(filters.action) : "Any action"}
								</Select.Trigger>
								<Select.Content>
									<Select.Item value="">Any action</Select.Item>
									{#each auditActions as action (action)}
										<Select.Item value={action}>{actionLabel(action)}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>

						<div class="flex flex-col gap-1.5">
							<Label for="audit-resource">Resource</Label>
							<Select.Root
								type="single"
								bind:value={filters.resourceKind}
								onValueChange={() => read()}
								disabled={busy}
							>
								<Select.Trigger id="audit-resource">
									{filters.resourceKind ? filters.resourceKind.replace(/_/g, " ") : "Any resource"}
								</Select.Trigger>
								<Select.Content>
									<Select.Item value="">Any resource</Select.Item>
									{#each resourceKinds as kind (kind)}
										<Select.Item value={kind}>{kind.replace(/_/g, " ")}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>

						<div class="flex flex-col gap-1.5">
							<Label for="audit-from">From</Label>
							<Input
								id="audit-from"
								type="date"
								bind:value={filters.from}
								onchange={() => read()}
								disabled={busy}
							/>
						</div>

						<div class="flex flex-col gap-1.5">
							<Label for="audit-to">To</Label>
							<Input
								id="audit-to"
								type="date"
								bind:value={filters.to}
								onchange={() => read()}
								disabled={busy}
							/>
						</div>
					</div>

					{#if filtersApplied(shown)}
						<div class="flex items-center gap-3">
							<Button variant="ghost" size="sm" onclick={clearFilters} disabled={busy}>
								Clear filters
							</Button>
						</div>
					{/if}
				</section>

				{#if record.kind === "empty"}
					<section
						class="flex flex-col items-center gap-2 rounded-lg border border-line-default bg-paper-0 px-6 py-12 text-center"
					>
						<ScrollText class="size-5 text-muted-foreground" aria-hidden="true" />
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							{filtersApplied(shown) ? "Nothing matches those filters" : "Nothing recorded yet"}
						</h2>
						<p class="max-w-100 text-sm leading-normal text-muted-foreground text-pretty">
							{#if filtersApplied(shown)}
								Widen the date range or clear the filters.
							{:else}
								Sign-ins, membership changes, tokens and configuration changes appear here as they
								happen.
							{/if}
							Records are kept for {retention} days.
						</p>
					</section>
				{:else}
					<div class="overflow-x-auto rounded-lg border border-line-default bg-paper-0">
						<table class="w-full min-w-160 border-collapse text-left text-sm">
							<thead>
								<tr class="border-b border-line-subtle text-xs text-muted-foreground">
									<th scope="col" class="px-4 py-2 font-medium">When</th>
									<th scope="col" class="px-4 py-2 font-medium">Action</th>
									<th scope="col" class="px-4 py-2 font-medium">Who</th>
									<th scope="col" class="px-4 py-2 font-medium">Resource</th>
									<th scope="col" class="px-4 py-2 font-medium">From</th>
								</tr>
							</thead>
							<tbody>
								{#each events as event (event.id)}
									<tr class="border-b border-line-subtle last:border-0 align-top">
										<td class="px-4 py-2.5 whitespace-nowrap text-muted-foreground">
											{occurred(event)}
										</td>
										<td class="px-4 py-2.5">
											<span class="text-ink-900">{actionLabel(event.action)}</span>
											{#if event.outcome !== "succeeded"}
												<span
													class="ml-1.5 rounded px-1.5 py-0.5 text-xs {event.outcome === 'denied'
														? 'bg-status-blocked/15 text-status-blocked'
														: 'bg-destructive/15 text-destructive'}"
												>
													{event.outcome}
												</span>
											{/if}
											{#if detailPairs(event).length > 0}
												<div class="mt-0.5 text-xs text-muted-foreground">
													{#each detailPairs(event) as [key, value], i (key)}
														{i > 0 ? " · " : ""}{key}: {value}
													{/each}
												</div>
											{/if}
										</td>
										<td class="px-4 py-2.5">
											<span class="text-ink-900">{actorLabel(event.actor)}</span>
											<div class="text-xs text-muted-foreground">{authLabel(event.actor)}</div>
										</td>
										<td class="px-4 py-2.5 text-muted-foreground">{resourceLabel(event)}</td>
										<td class="px-4 py-2.5 whitespace-nowrap text-muted-foreground">
											{event.sourceIp ?? "—"}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>

					<div class="flex items-center justify-between gap-3">
						<p class="text-xs text-muted-foreground">Records are kept for {retention} days.</p>
						{#if record.kind === "ready" && record.nextCursor}
							<Button
								variant="secondary"
								size="sm"
								disabled={busy}
								onclick={() => read(record.kind === "ready" ? record.nextCursor : undefined)}
							>
								{busy ? "Loading…" : "Load more"}
							</Button>
						{/if}
					</div>
				{/if}
			{/if}
		</div>
	</div>
</div>
