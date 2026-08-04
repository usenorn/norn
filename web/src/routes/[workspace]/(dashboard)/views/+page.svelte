<script lang="ts">
	import { goto, invalidateAll } from "$app/navigation";
	import { page } from "$app/state";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronUp from "@lucide/svelte/icons/chevron-up";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Bookmark from "@lucide/svelte/icons/bookmark";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Select from "$lib/components/ui/select/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { api } from "$lib/api";
	import { viewSchema } from "$lib/views/view-schema";
	import {
		listingFor,
		reordered,
		scopeOf,
		sharingLabels,
		sharingNotes,
		sharings,
		viewFailureMessage,
		viewPath,
		viewsOf,
		readViewFailure,
		type SavedView,
		type SavedViewSharing,
		type ViewFailure,
		type ViewListing,
	} from "$lib/views/views";
	import { workspacePath } from "$lib/workspace/navigation";
	import { viewsPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "saved-view-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? viewsPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let submitted = $state<ViewListing | null>(null);
	let localFailure = $state<ViewFailure | null>(null);
	let localWorking = $state("");
	let announcement = $state("");

	const slug = $derived(data.workspace.slug);
	const listing = $derived<ViewListing>(submitted ?? preview?.listing ?? data.listing);
	const views = $derived(viewsOf(listing));
	const teams = $derived(preview?.teams ?? data.teams);
	const failure = $derived<ViewFailure | null>(preview?.failure ?? localFailure);
	const working = $derived(preview?.working ?? localWorking);

	const removalId = $derived(preview?.removing ?? page.url.searchParams.get("remove") ?? "");
	const removing = $derived(views.find((view) => view.id === removalId) ?? null);
	const editingId = $derived(preview?.editing ?? page.url.searchParams.get("edit") ?? "");
	const editing = $derived(views.find((view) => view.id === editingId) ?? null);

	async function refresh() {
		const { data: fresh } = await api.GET("/workspaces/{workspaceId}/saved-views", {
			params: { path: { workspaceId: data.workspace.id } },
		});

		submitted = listingFor(fresh);
		await invalidateAll();
	}

	async function closePanels() {
		const next = new URL(page.url);
		next.searchParams.delete("remove");
		next.searchParams.delete("edit");
		localFailure = null;

		await goto(next, { replaceState: true, noScroll: true, keepFocus: true });
	}

	function panelHref(key: "remove" | "edit", view: SavedView): string {
		const next = new URL(page.url);
		next.searchParams.delete(key === "remove" ? "edit" : "remove");
		next.searchParams.set(key, view.id);

		return `${next.pathname}${next.search}`;
	}

	const form = superForm(defaults(zod4(viewSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(viewSchema),
		resetForm: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid || !editing) return;

			localFailure = null;

			const body = {
				name: pending.data.name,
				sharing: pending.data.sharing as SavedViewSharing,
				...(pending.data.sharing === "team" ? { teamId: pending.data.teamId } : {}),
			};

			try {
				const { data: saved, error } = await api.PATCH(
					"/workspaces/{workspaceId}/saved-views/{savedViewId}",
					{
						params: { path: { workspaceId: data.workspace.id, savedViewId: editing.id } },
						body,
					}
				);

				if (error || !saved) {
					localFailure = readViewFailure(error);

					return;
				}

				announcement = `${saved.name} was saved.`;
				await closePanels();
				await refresh();
			} catch {
				localFailure = { kind: "unavailable" };
			}
		},
	});

	const { form: formData, enhance, submitting } = form;

	const busy = $derived(working !== "" || $submitting);

	$effect(() => {
		const view = editing;
		if (!view) return;

		formData.set(
			{ name: view.name, sharing: view.sharing, teamId: view.teamId ?? "" },
			{ taint: false }
		);
	});

	async function confirmRemoval() {
		if (!removing) return;

		localWorking = removing.id;
		localFailure = null;

		try {
			const { error } = await api.DELETE(
				"/workspaces/{workspaceId}/saved-views/{savedViewId}",
				{
					params: {
						path: { workspaceId: data.workspace.id, savedViewId: removing.id },
						query: { acknowledgedSharing: removing.sharing },
					},
				}
			);

			if (error) {
				localFailure = readViewFailure(error);

				return;
			}

			announcement = `${removing.name} was removed.`;
			await closePanels();
			await refresh();
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			localWorking = "";
		}
	}

	async function move(view: SavedView, delta: number) {
		const savedViewIds = reordered(views, view.id, delta);
		if (savedViewIds.every((id, index) => id === views[index].id)) return;

		localWorking = view.id;
		localFailure = null;

		try {
			const { data: ordered, error } = await api.PUT(
				"/workspaces/{workspaceId}/saved-views/order",
				{ params: { path: { workspaceId: data.workspace.id } }, body: { savedViewIds } }
			);

			if (error || !ordered) {
				localFailure = readViewFailure(error);

				return;
			}

			submitted = listingFor(ordered);
			announcement = `${view.name} moved to position ${savedViewIds.indexOf(view.id) + 1} of ${savedViewIds.length}.`;
			await invalidateAll();
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			localWorking = "";
		}
	}

	function removalCopy(view: SavedView): string {
		switch (view.sharing) {
			case "team":
				return `${view.name} is shared with ${view.teamName ?? "a team"}. Removing it takes it off the sidebar for everyone on that team.`;
			case "workspace":
				return `${view.name} is shared with everyone in ${data.workspace.name}. Removing it takes it off their sidebar too.`;
			default:
				return `Remove ${view.name}? It is only yours, so nothing else changes.`;
		}
	}

	function removalLabel(view: SavedView): string {
		switch (view.sharing) {
			case "team":
				return `Remove for ${view.teamName ?? "the team"}`;
			case "workspace":
				return "Remove for everyone";
			default:
				return "Remove view";
		}
	}
</script>

<svelte:head><title>Views · {data.workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex-none border-b border-line-default">
		<div class="flex h-11 items-center gap-2 pr-3 pl-4">
			<Bookmark class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
			<h1 class="text-md font-medium tracking-snug whitespace-nowrap text-ink-900">Views</h1>
		</div>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-140 flex-col gap-6 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			<p class="sr-only" role="status" aria-live="polite">{announcement}</p>

			{#if failure}
				<Alert.Root variant="destructive">
					<CircleX aria-hidden="true" />
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>{viewFailureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if listing.kind === "loading"}
				<div class="h-40 animate-pulse rounded-lg bg-paper-2" aria-busy="true"></div>
			{:else if listing.kind === "unavailable"}
				<p class="text-sm leading-normal text-muted-foreground">
					We could not load your saved views. Nothing changed &mdash; wait a moment and try again.
				</p>
			{:else if listing.kind === "empty"}
				<div class="flex flex-col gap-3">
					<p class="text-md font-medium tracking-snug text-ink-900">No saved views yet</p>
					<p class="max-w-100 text-md leading-normal text-muted-foreground text-pretty">
						Set up the issues screen the way you want it &mdash; a team, a tab, a grouping &mdash;
						then save it. Keep it to yourself, or share it with a team so everyone opens the same
						question through their own permissions.
					</p>
					<div>
						<Button href={workspacePath(slug, "/issues")} variant="secondary" size="sm">
							Go to issues
						</Button>
					</div>
				</div>
			{:else}
				<ul class="flex flex-col divide-y divide-line-subtle rounded-lg border border-line-default">
					{#each views as view, index (view.id)}
						<li class="flex flex-col">
							<div class="flex flex-wrap items-center gap-2 px-3 py-2">
								<a
									href={viewPath(slug, view)}
									class="min-w-0 flex-[1_1_140px] truncate text-md text-ink-900 hover:text-link"
								>
									{view.name}
								</a>
								<span class="shrink-0 font-mono text-2xs tracking-eyebrow text-ink-600 uppercase">
									{scopeOf(view)}
								</span>
								<div class="flex shrink-0 items-center gap-1">
									<Button
										variant="ghost"
										size="icon-sm"
										disabled={busy || index === 0}
										aria-label="Move {view.name} up"
										onclick={() => move(view, -1)}
									>
										<ChevronUp aria-hidden="true" />
									</Button>
									<Button
										variant="ghost"
										size="icon-sm"
										disabled={busy || index === views.length - 1}
										aria-label="Move {view.name} down"
										onclick={() => move(view, 1)}
									>
										<ChevronDown aria-hidden="true" />
									</Button>
									{#if view.editable}
										<Button variant="ghost" size="sm" href={panelHref("edit", view)}>Edit</Button>
										<Button variant="ghost" size="sm" href={panelHref("remove", view)}>
											Remove
										</Button>
									{:else}
										<span class="px-2 text-sm text-muted-foreground">
											{view.createdByName ?? "Someone else"}
										</span>
									{/if}
								</div>
							</div>

							{#if editing?.id === view.id}
								<div class="flex flex-col gap-4 border-t border-line-subtle bg-paper-2 px-3 py-4">
									<form method="POST" id={formId} use:enhance class="flex flex-col gap-4">
										<Form.Field {form} name="name">
											<Form.Control>
												{#snippet children({ props })}
													<Form.Label>Name</Form.Label>
													<Input {...props} disabled={busy} bind:value={$formData.name} />
												{/snippet}
											</Form.Control>
											<Form.FieldErrors />
										</Form.Field>

										<Form.Field {form} name="sharing">
											<Form.Control>
												{#snippet children({ props })}
													<Form.Label>Who sees it</Form.Label>
													<Select.Root type="single" bind:value={$formData.sharing} disabled={busy}>
														<Select.Trigger {...props}>
															{sharingLabels[$formData.sharing]}
														</Select.Trigger>
														<Select.Content>
															{#each sharings as sharing (sharing)}
																<Select.Item value={sharing} label={sharingLabels[sharing]}>
																	{sharingLabels[sharing]}
																</Select.Item>
															{/each}
														</Select.Content>
													</Select.Root>
												{/snippet}
											</Form.Control>
											<Form.Description>{sharingNotes[$formData.sharing]}</Form.Description>
											<Form.FieldErrors />
										</Form.Field>

										{#if $formData.sharing === "team"}
											<Form.Field {form} name="teamId">
												<Form.Control>
													{#snippet children({ props })}
														<Form.Label>Team</Form.Label>
														<Select.Root
															type="single"
															bind:value={$formData.teamId}
															disabled={busy}
														>
															<Select.Trigger {...props}>
																{teams.find((team) => team.id === $formData.teamId)?.name ??
																	"Choose a team"}
															</Select.Trigger>
															<Select.Content>
																{#each teams as team (team.id)}
																	<Select.Item value={team.id} label={team.name}>
																		{team.name}
																	</Select.Item>
																{/each}
															</Select.Content>
														</Select.Root>
													{/snippet}
												</Form.Control>
												<Form.FieldErrors />
											</Form.Field>
										{/if}
									</form>
									<div class="flex flex-wrap gap-2">
										<Button type="submit" form={formId} size="sm" disabled={busy}>
											{$submitting ? "Saving" : "Save changes"}
										</Button>
										<Button variant="secondary" size="sm" disabled={busy} onclick={closePanels}>
											Cancel
										</Button>
									</div>
								</div>
							{/if}

							{#if removing?.id === view.id}
								<div class="flex flex-col gap-3 border-t border-line-subtle bg-paper-2 px-3 py-4">
									<p class="text-md leading-normal text-ink-900 text-pretty">
										{removalCopy(view)}
									</p>
									<div class="flex flex-wrap gap-2">
										<Button
											variant="destructive"
											size="sm"
											disabled={busy}
											onclick={confirmRemoval}
										>
											{working === view.id ? "Removing" : removalLabel(view)}
										</Button>
										<Button variant="secondary" size="sm" disabled={busy} onclick={closePanels}>
											Keep it
										</Button>
									</div>
								</div>
							{/if}
						</li>
					{/each}
				</ul>

				<p class="text-sm leading-normal text-muted-foreground text-pretty">
					This order is yours alone. Rearranging a shared view here never moves it in anyone else's
					sidebar.
				</p>
			{/if}
		</div>
	</div>
</div>
