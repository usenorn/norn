<script lang="ts">
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import Webhook from "@lucide/svelte/icons/webhook";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { onDateAndTime } from "$lib/time";
	import { createWebhookSchema } from "$lib/webhooks/webhook-schema";
	import {
		disableReasonLabel,
		eventCatalog,
		eventGroups,
		eventLabel,
		failureMessage,
		webhookFailure,
		webhookPath,
		type WebhookFailure,
		type WebhooksView,
	} from "$lib/webhooks/webhooks";
	import { webhooksPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "create-webhook-form";
	const shownEvents = 4;

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? webhooksPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let loaded = $state.raw<WebhooksView | null>(null);
	let localFailure = $state.raw<WebhookFailure | null>(null);
	let copied = $state(false);

	const workspace = $derived(page.data.workspace);
	const view = $derived<WebhooksView>(loaded ?? preview?.view ?? data.view);
	const failure = $derived(preview?.failure ?? localFailure);
	const current = $derived(view.kind === "list" || view.kind === "created" ? view.webhooks : []);
	const created = $derived(view.kind === "created" ? view : null);
	const composable = $derived(
		view.kind === "empty" || view.kind === "list" || view.kind === "created"
	);

	const form = superForm(defaults(zod4(createWebhookSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(createWebhookSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			localFailure = null;
			copied = false;

			try {
				const { data: minted, error } = await api.POST("/workspaces/{workspaceId}/webhooks", {
					params: { path: { workspaceId: workspace.id } },
					body: {
						name: entered.data.name,
						url: entered.data.url,
						events: eventCatalog.filter((event) => entered.data.events.includes(event)),
					},
				});

				if (error) {
					const mapped = webhookFailure(error);

					if (mapped.kind === "destination_refused") {
						setError(entered, "url", "This instance will not post to that address.");
					}

					localFailure = mapped;

					return;
				}

				if (!minted) {
					localFailure = { kind: "unavailable" };

					return;
				}

				loaded = {
					kind: "created",
					webhooks: [minted.webhook, ...current],
					webhook: minted.webhook,
					secret: minted.secret,
				};

				formData.set({ name: "", url: "", events: [] });
			} catch {
				localFailure = { kind: "unavailable" };
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	const busy = $derived(preview?.busy || $submitting);

	function toggleEvent(event: string, checked: boolean) {
		const next = new Set($formData.events);

		if (checked) {
			next.add(event);
		} else {
			next.delete(event);
		}

		formData.update((entered) => ({ ...entered, events: [...next] }));
	}

	function toggleGroup(events: string[], checked: boolean) {
		const next = new Set($formData.events);

		for (const event of events) {
			if (checked) {
				next.add(event);
			} else {
				next.delete(event);
			}
		}

		formData.update((entered) => ({ ...entered, events: [...next] }));
	}

	async function copySecret() {
		if (!created) return;

		await navigator.clipboard.writeText(created.secret);
		copied = true;
	}
</script>

<svelte:head><title>Webhooks · {workspace.name} · Norn</title></svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center gap-2 border-b border-line-subtle px-4">
		<Webhook class="size-4 text-muted-foreground" aria-hidden="true" />
		<h1 class="text-sm font-medium tracking-snug text-ink-900">Webhooks</h1>
	</div>

	<div class="relative flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-180 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if failure}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>{failureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if view.kind === "signing_unavailable"}
				<section class="flex flex-col gap-3 rounded-lg border border-line-default bg-paper-0 p-5">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							This instance cannot sign a webhook yet
						</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Every request Norn sends carries a signature, and the secret behind it is stored
							encrypted. Without an encryption key there is nothing to sign with and nowhere safe to
							keep a secret, so subscriptions cannot be created.
						</p>
					</div>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						An operator sets <span class="font-mono text-ink-900">NORN_SECURITY_ENCRYPTION_KEY</span>
						on the instance and restarts it. Nothing else here changes, and events raised meanwhile are
						not kept waiting.
					</p>
				</section>
			{:else if view.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not manage webhooks here</Alert.Title>
					<Alert.Description>Ask an administrator of {workspace.name}.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load the webhooks</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "loading"}
				<p class="text-sm text-muted-foreground">Loading webhooks…</p>
			{/if}

			{#if created}
				<section class="flex flex-col gap-3 rounded-lg border border-success/40 bg-paper-0 p-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">
							Copy the signing secret for {created.webhook.name} now
						</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							This is the only time it is shown. Your endpoint uses it to verify that a request
							really came from Norn. If you lose it, replace it from the subscription.
						</p>
					</div>
					<p class="rounded-md bg-paper-100 p-3 font-mono text-xs break-all text-ink-900">
						{created.secret}
					</p>
					<div class="flex flex-wrap gap-2">
						<Button variant="secondary" size="sm" onclick={copySecret}>
							{copied ? "Copied" : "Copy secret"}
						</Button>
						<Button
							variant="ghost"
							size="sm"
							href={webhookPath(workspace.slug, created.webhook.id)}
						>
							Open the subscription
						</Button>
					</div>
				</section>
			{/if}

			{#if composable}
				<section class="flex flex-col gap-3">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Subscriptions</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Norn posts each event you pick to your address, signed with a secret only you and this
							instance hold. Deliveries that keep failing are retried, then the subscription is
							turned off rather than left hammering a dead endpoint.
						</p>
					</div>

					{#if current.length === 0}
						<div
							class="flex flex-col items-center gap-2 rounded-lg border border-line-default bg-paper-0 px-6 py-10 text-center"
						>
							<Webhook class="size-5 text-muted-foreground" aria-hidden="true" />
							<p class="max-w-100 text-sm leading-normal text-muted-foreground text-pretty">
								Nothing is subscribed yet. Add an address below and Norn starts posting the events
								you choose as they happen.
							</p>
						</div>
					{:else}
						<ul class="rounded-lg border border-line-subtle bg-paper-0">
							{#each current as webhook (webhook.id)}
								<li class="border-b border-line-subtle last:border-b-0">
									<a
										href={webhookPath(workspace.slug, webhook.id)}
										class="flex flex-col gap-1.5 p-3 motion-control hover:bg-accent"
									>
										<div class="flex flex-wrap items-center gap-2">
											<span class="truncate text-sm text-ink-900">{webhook.name}</span>
											{#if webhook.enabled}
												<Tag name="On" />
											{:else}
												<Tag name="Off" color="magenta" />
											{/if}
										</div>
										<p class="font-mono text-xs break-all text-muted-foreground">{webhook.url}</p>
										<div class="flex flex-wrap items-center gap-1">
											{#each webhook.events.slice(0, shownEvents) as event (event)}
												<Tag name={eventLabel(event)} />
											{/each}
											{#if webhook.events.length > shownEvents}
												<span class="text-xs text-muted-foreground">
													and {webhook.events.length - shownEvents} more
												</span>
											{/if}
										</div>
										<p class="text-xs text-muted-foreground">
											{#if webhook.lastDeliveredAt}
												Last delivered {onDateAndTime(webhook.lastDeliveredAt, workspace.timezone)}
											{:else}
												Nothing delivered yet
											{/if}
										</p>
										{#if webhook.failureStreak > 0}
											<p class="text-xs text-destructive">
												{webhook.failureStreak} delivery{webhook.failureStreak === 1 ? "" : "s"} in a
												row failed
											</p>
										{/if}
										{#if !webhook.enabled && webhook.disabledReason}
											<p class="text-xs text-muted-foreground">
												{disableReasonLabel(webhook.disabledReason)}
											</p>
										{/if}
									</a>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Add a subscription</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							The address has to be reachable from the internet. Norn refuses addresses inside its
							own network, so a subscription cannot be used to reach something private.
						</p>
					</div>

					<Form.Field {form} name="name">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Name</Form.Label>
								<Input
									{...props}
									bind:value={$formData.name}
									disabled={busy}
									placeholder="Deploy pipeline"
									autocomplete="off"
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="url">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Destination</Form.Label>
								<Input
									{...props}
									bind:value={$formData.url}
									disabled={busy}
									inputmode="url"
									placeholder="https://example.com/hooks/norn"
									autocomplete="off"
									spellcheck="false"
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="events">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Events</Form.Label>
								<div {...props} class="flex flex-col gap-3">
									{#each eventGroups as group (group.subject)}
										{@const all = group.events.every((event) => $formData.events.includes(event))}
										<div class="rounded-md border border-line-subtle p-3">
											<div class="flex items-center gap-2">
												<Checkbox
													id={`event-group-${group.subject}`}
													disabled={busy}
													checked={all}
													onCheckedChange={(checked) => toggleGroup(group.events, checked === true)}
												/>
												<label
													for={`event-group-${group.subject}`}
													class="text-sm font-medium leading-normal tracking-snug text-ink-900"
												>
													{group.label}
												</label>
											</div>

											<div class="mt-3 flex flex-col gap-1.5 pl-6">
												{#each group.events as event (event)}
													<div class="flex items-start gap-2">
														<Checkbox
															id={`event-${event}`}
															disabled={busy}
															checked={$formData.events.includes(event)}
															onCheckedChange={(checked) => toggleEvent(event, checked === true)}
														/>
														<label
															for={`event-${event}`}
															class="text-sm leading-normal text-pretty text-ink-600"
														>
															{eventLabel(event)}
															<span class="font-mono text-xs text-muted-foreground">{event}</span>
														</label>
													</div>
												{/each}
											</div>
										</div>
									{/each}
								</div>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<div>
						<Form.Button disabled={busy}>
							{$submitting ? "Adding…" : "Add subscription"}
						</Form.Button>
					</div>
				</form>
			{/if}
		</div>
	</div>
</div>
