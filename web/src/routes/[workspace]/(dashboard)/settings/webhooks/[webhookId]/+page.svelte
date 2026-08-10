<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import ChevronDown from "@lucide/svelte/icons/chevron-down";
	import ChevronRight from "@lucide/svelte/icons/chevron-right";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as Table from "$lib/components/ui/table/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import Tag from "$lib/components/norn/tag.svelte";
	import { api } from "$lib/api";
	import { onDateAndTime } from "$lib/time";
	import { editWebhookSchema } from "$lib/webhooks/webhook-schema";
	import {
		deliverySummary,
		disableReasonLabel,
		eventCatalog,
		eventGroups,
		eventLabel,
		failureMessage,
		outcomeLabel,
		stateLabel,
		webhookFailure,
		webhooksPath,
		type WebhookAttempt,
		type WebhookDetailView,
		type WebhookDeliveryDetail,
		type WebhookFailure,
	} from "$lib/webhooks/webhooks";
	import { webhookDetailPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "edit-webhook-form";
	const pageSize = 25;
	const logColumns = 7;

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? webhookDetailPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let loaded = $state.raw<WebhookDetailView | null>(null);
	let localFailure = $state.raw<WebhookFailure | null>(null);
	let fetched = $state.raw<Record<string, WebhookDeliveryDetail>>({});
	let opened = $state.raw<string | null>(null);
	let working = $state(false);
	let replaying = $state.raw<string | null>(null);
	let confirming = $state(false);
	let copied = $state(false);

	const workspace = $derived(page.data.workspace);
	const view = $derived<WebhookDetailView>(loaded ?? preview?.view ?? data.view);
	const failure = $derived(preview?.failure ?? localFailure);
	const busy = $derived(preview?.busy ?? working);
	const live = $derived(view.kind === "detail" ? view : null);
	const expanded = $derived(preview?.expanded ?? opened);
	const details = $derived({ ...(preview?.details ?? {}), ...fetched });

	const form = superForm(defaults(zod4(editWebhookSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(editWebhookSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid || !live) return;

			localFailure = null;

			try {
				const { data: updated, error } = await api.PATCH(
					"/workspaces/{workspaceId}/webhooks/{webhookId}",
					{
						params: { path: { workspaceId: workspace.id, webhookId: live.webhook.id } },
						body: {
							url: entered.data.url,
							events: eventCatalog.filter((event) => entered.data.events.includes(event)),
						},
					}
				);

				if (error) {
					const mapped = webhookFailure(error);

					if (mapped.kind === "destination_refused") {
						setError(entered, "url", "This instance will not post to that address.");
					}

					localFailure = mapped;

					return;
				}

				if (!updated) {
					localFailure = { kind: "unavailable" };

					return;
				}

				loaded = { ...live, webhook: updated, secret: undefined };
			} catch {
				localFailure = { kind: "unavailable" };
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	const storedUrl = $derived(live?.webhook.url);
	const storedEvents = $derived(live ? live.webhook.events.join(" ") : undefined);

	$effect(() => {
		if (storedUrl === undefined || storedEvents === undefined) return;

		const events = storedEvents ? storedEvents.split(" ") : [];

		formData.update((entered) => ({ ...entered, url: storedUrl, events }), { taint: false });
	});

	const pending = $derived(busy || $submitting || replaying !== null);

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

	async function setEnabled(enabled: boolean) {
		if (!live) return;

		working = true;
		localFailure = null;

		try {
			const { data: updated, error } = await api.PATCH(
				"/workspaces/{workspaceId}/webhooks/{webhookId}",
				{
					params: { path: { workspaceId: workspace.id, webhookId: live.webhook.id } },
					body: { enabled },
				}
			);

			if (error || !updated) {
				localFailure = error ? webhookFailure(error) : { kind: "unavailable" };

				return;
			}

			loaded = { ...live, webhook: updated, secret: undefined, lastAttempt: undefined };
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function rotate() {
		if (!live) return;

		working = true;
		localFailure = null;
		copied = false;

		try {
			const { data: minted, error } = await api.POST(
				"/workspaces/{workspaceId}/webhooks/{webhookId}/secret",
				{ params: { path: { workspaceId: workspace.id, webhookId: live.webhook.id } } }
			);

			if (error || !minted) {
				localFailure = error ? webhookFailure(error) : { kind: "unavailable" };

				return;
			}

			loaded = { ...live, webhook: minted.webhook, secret: minted.secret };
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function sendTest() {
		if (!live) return;

		working = true;
		localFailure = null;

		try {
			const { data: queued, error } = await api.POST(
				"/workspaces/{workspaceId}/webhooks/{webhookId}/test",
				{ params: { path: { workspaceId: workspace.id, webhookId: live.webhook.id } } }
			);

			if (error || !queued) {
				localFailure = error ? webhookFailure(error) : { kind: "unavailable" };

				return;
			}

			loaded = { ...live, deliveries: [queued, ...live.deliveries], secret: undefined };
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function remove() {
		if (!live) return;

		working = true;
		localFailure = null;

		try {
			const { error } = await api.DELETE("/workspaces/{workspaceId}/webhooks/{webhookId}", {
				params: { path: { workspaceId: workspace.id, webhookId: live.webhook.id } },
			});

			if (error) {
				localFailure = webhookFailure(error);

				return;
			}

			await goto(webhooksPath(workspace.slug));
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function inspect(deliveryId: string) {
		if (expanded === deliveryId) {
			opened = null;

			return;
		}

		opened = deliveryId;

		if (!live || details[deliveryId]) return;

		const { data: found } = await api.GET(
			"/workspaces/{workspaceId}/webhooks/{webhookId}/deliveries/{deliveryId}",
			{
				params: {
					path: { workspaceId: workspace.id, webhookId: live.webhook.id, deliveryId },
				},
			}
		);

		if (found) fetched = { ...fetched, [deliveryId]: found };
	}

	async function replay(deliveryId: string) {
		if (!live) return;

		replaying = deliveryId;
		localFailure = null;

		try {
			const { data: queued, error } = await api.POST(
				"/workspaces/{workspaceId}/webhooks/{webhookId}/deliveries/{deliveryId}/replay",
				{
					params: {
						path: { workspaceId: workspace.id, webhookId: live.webhook.id, deliveryId },
					},
				}
			);

			if (error || !queued) {
				localFailure = error ? webhookFailure(error) : { kind: "unavailable" };

				return;
			}

			loaded = { ...live, deliveries: [queued, ...live.deliveries], secret: undefined };
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			replaying = null;
		}
	}

	async function more() {
		if (!live?.nextCursor) return;

		working = true;

		try {
			const { data: found, error } = await api.GET(
				"/workspaces/{workspaceId}/webhooks/{webhookId}/deliveries",
				{
					params: {
						path: { workspaceId: workspace.id, webhookId: live.webhook.id },
						query: { limit: pageSize, cursor: live.nextCursor },
					},
				}
			);

			if (error || !found) {
				localFailure = error ? webhookFailure(error) : { kind: "unavailable" };

				return;
			}

			loaded = {
				...live,
				deliveries: [...live.deliveries, ...found.deliveries],
				nextCursor: found.nextCursor,
				secret: undefined,
			};
		} catch {
			localFailure = { kind: "unavailable" };
		} finally {
			working = false;
		}
	}

	async function copySecret() {
		if (!live?.secret) return;

		await navigator.clipboard.writeText(live.secret);
		copied = true;
	}

	function lastAttemptOf(deliveryId: string): WebhookAttempt | undefined {
		return details[deliveryId]?.attempts.at(-1);
	}

	function duration(elapsedMs: number): string {
		return elapsedMs < 1000 ? `${elapsedMs} ms` : `${(elapsedMs / 1000).toFixed(1)} s`;
	}

	function attemptLine(attempt: WebhookAttempt): string {
		const code = attempt.statusCode ? ` · ${attempt.statusCode}` : "";

		return `${outcomeLabel(attempt.outcome)}${code}${attempt.error ? ` · ${attempt.error}` : ""}`;
	}
</script>

<svelte:head>
	<title>{live?.webhook.name ?? "Webhook"} · {workspace.name} · Norn</title>
</svelte:head>

<div class="flex min-h-0 flex-1 flex-col">
	<div class="flex h-11 flex-none items-center gap-3 border-b border-line-subtle px-4">
		<a
			href={webhooksPath(workspace.slug)}
			class="flex items-center gap-1.5 text-sm text-muted-foreground motion-control hover:text-ink-900"
		>
			<ArrowLeft class="size-4" aria-hidden="true" />
			<span>Webhooks</span>
		</a>
	</div>

	<div class="flex-1 overflow-auto">
		<div
			class="mx-auto flex w-full max-w-240 flex-col gap-5 px-4 py-6 pb-[calc(--spacing(10)+env(safe-area-inset-bottom))]"
		>
			{#if failure}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That did not work</Alert.Title>
					<Alert.Description>{failureMessage(failure)}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if view.kind === "not_found"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>That subscription is not here</Alert.Title>
					<Alert.Description>It may already have been removed.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "forbidden"}
				<Alert.Root variant="muted">
					<CircleAlert aria-hidden="true" />
					<Alert.Title>You may not manage webhooks here</Alert.Title>
					<Alert.Description>Ask an administrator of {workspace.name}.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "unavailable"}
				<Alert.Root variant="destructive">
					<TriangleAlert aria-hidden="true" />
					<Alert.Title>Could not load this subscription</Alert.Title>
					<Alert.Description>Check your connection and reload.</Alert.Description>
				</Alert.Root>
			{:else if view.kind === "loading"}
				<p class="text-sm text-muted-foreground">Loading this subscription…</p>
			{:else if live}
				<section class="flex flex-col gap-2">
					<div class="flex flex-wrap items-center gap-2">
						<h1 class="text-lg font-medium tracking-snug text-ink-900">{live.webhook.name}</h1>
						{#if live.webhook.enabled}
							<Tag name="On" />
						{:else}
							<Tag name="Off" color="magenta" />
						{/if}
					</div>
					<p class="font-mono text-xs break-all text-muted-foreground">{live.webhook.url}</p>
					<p class="text-sm text-muted-foreground">
						Signing secret {live.webhook.secretHint}
						{#if live.webhook.lastDeliveredAt}
							· last delivered {onDateAndTime(live.webhook.lastDeliveredAt, workspace.timezone)}
						{:else}
							· nothing delivered yet
						{/if}
					</p>
				</section>

				{#if !live.webhook.enabled}
					<Alert.Root variant="warning">
						<TriangleAlert aria-hidden="true" />
						<Alert.Title>
							{live.webhook.disabledReason
								? disableReasonLabel(live.webhook.disabledReason)
								: "This subscription is turned off"}
						</Alert.Title>
						<Alert.Description>
							Nothing is being sent, and events raised while it is off are not held for it.
							{#if live.webhook.disabledAt}
								Turned off {onDateAndTime(live.webhook.disabledAt, workspace.timezone)}.
							{/if}
							{#if live.lastAttempt}
								The last attempt reached {live.lastAttempt.requestUrl} and {attemptLine(
									live.lastAttempt
								)}.
							{/if}
						</Alert.Description>
						<Alert.Action>
							<Button
								variant="secondary"
								size="sm"
								disabled={pending}
								onclick={() => setEnabled(true)}
							>
								Resume
							</Button>
						</Alert.Action>
					</Alert.Root>
				{/if}

				{#if live.secret}
					<section class="flex flex-col gap-3 rounded-lg border border-success/40 bg-paper-0 p-4">
						<div class="flex flex-col gap-1">
							<h2 class="text-md font-medium tracking-snug text-ink-900">
								Copy the new signing secret now
							</h2>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								This is the only time it is shown. The old secret stops working immediately, so
								update your endpoint before the next event arrives.
							</p>
						</div>
						<p class="rounded-md bg-paper-100 p-3 font-mono text-xs break-all text-ink-900">
							{live.secret}
						</p>
						<div>
							<Button variant="secondary" size="sm" onclick={copySecret}>
								{copied ? "Copied" : "Copy secret"}
							</Button>
						</div>
					</section>
				{/if}

				<form method="POST" id={formId} use:enhance class="flex flex-col gap-5">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">What is sent, and where</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Changing the destination or the events takes effect on the next event raised.
							Deliveries already queued keep the address they were queued with.
						</p>
					</div>

					<Form.Field {form} name="url">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Destination</Form.Label>
								<Input
									{...props}
									bind:value={$formData.url}
									disabled={pending}
									inputmode="url"
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
													disabled={pending}
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
															disabled={pending}
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
						<Form.Button disabled={pending}>
							{$submitting ? "Saving…" : "Save changes"}
						</Form.Button>
					</div>
				</form>

				<section class="flex flex-col gap-4 rounded-lg border border-line-subtle p-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Secret and state</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Replacing the secret shows it once and invalidates the old one straight away. Turning
							the subscription off stops delivery without losing what is already recorded.
						</p>
					</div>

					<div class="flex flex-wrap gap-2">
						<Button variant="secondary" size="sm" disabled={pending} onclick={rotate}>
							Replace the secret
						</Button>
						<Button
							variant="secondary"
							size="sm"
							disabled={pending || !live.webhook.enabled}
							onclick={sendTest}
						>
							Send a test event
						</Button>
						<Button
							variant="ghost"
							size="sm"
							disabled={pending}
							onclick={() => setEnabled(!live.webhook.enabled)}
						>
							{live.webhook.enabled ? "Turn off" : "Resume"}
						</Button>
					</div>
				</section>

				<section class="flex flex-col gap-4 rounded-lg border border-destructive/40 p-4">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Remove this subscription</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Removing it stops delivery at once and takes its delivery log with it. Turning it off
							instead keeps the record.
						</p>
					</div>

					{#if confirming}
						<div class="flex flex-wrap items-center gap-2">
							<p class="text-sm text-ink-900">Remove {live.webhook.name} and its delivery log?</p>
							<Button variant="destructive" size="sm" disabled={pending} onclick={remove}>
								{working ? "Removing…" : "Yes, remove it"}
							</Button>
							<Button
								variant="ghost"
								size="sm"
								disabled={pending}
								onclick={() => (confirming = false)}
							>
								Keep it
							</Button>
						</div>
					{:else}
						<div>
							<Button
								variant="secondary"
								size="sm"
								disabled={pending}
								onclick={() => (confirming = true)}
							>
								Remove subscription
							</Button>
						</div>
					{/if}
				</section>

				<section class="flex flex-col gap-3">
					<div class="flex flex-col gap-1">
						<h2 class="text-md font-medium tracking-snug text-ink-900">Delivery log</h2>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Every attempt Norn made, newest first. Open one to see the exact request, what came
							back, and the payload that was sent.
						</p>
					</div>

					{#if live.deliveries.length === 0}
						<div
							class="flex flex-col items-center gap-2 rounded-lg border border-line-default bg-paper-0 px-6 py-10 text-center"
						>
							<p class="max-w-100 text-sm leading-normal text-muted-foreground text-pretty">
								Nothing has been sent yet. Send a test event to check the endpoint before the first
								real one arrives.
							</p>
						</div>
					{:else}
						<Table.Root class="min-w-160">
							<Table.Header>
								<Table.Row>
									<Table.Head>Event</Table.Head>
									<Table.Head>When</Table.Head>
									<Table.Head>State</Table.Head>
									<Table.Head>Attempt</Table.Head>
									<Table.Head>Outcome</Table.Head>
									<Table.Head>Status</Table.Head>
									<Table.Head>Duration</Table.Head>
								</Table.Row>
							</Table.Header>
							<Table.Body>
								{#each live.deliveries as delivery (delivery.id)}
									{@const attempt = lastAttemptOf(delivery.id)}
									{@const open = expanded === delivery.id}
									<Table.Row>
										<Table.Cell>
											<button
												type="button"
												class="flex items-center gap-1.5 text-left text-ink-900"
												onclick={() => inspect(delivery.id)}
												aria-expanded={open}
												aria-controls={`delivery-${delivery.id}`}
											>
												{#if open}
													<ChevronDown class="size-3.5 text-muted-foreground" aria-hidden="true" />
												{:else}
													<ChevronRight class="size-3.5 text-muted-foreground" aria-hidden="true" />
												{/if}
												{eventLabel(delivery.event)}
											</button>
										</Table.Cell>
										<Table.Cell class="text-muted-foreground">
											{onDateAndTime(delivery.createdAt, workspace.timezone)}
										</Table.Cell>
										<Table.Cell>
											<span
												class="rounded px-1.5 py-0.5 text-xs {delivery.state === 'succeeded'
													? 'bg-success/15 text-success'
													: delivery.state === 'pending'
														? 'bg-status-blocked/15 text-status-blocked'
														: 'bg-destructive/15 text-destructive'}"
											>
												{stateLabel(delivery.state)}
											</span>
										</Table.Cell>
										<Table.Cell class="text-muted-foreground">{delivery.attempt}</Table.Cell>
										<Table.Cell class="text-muted-foreground">
											{attempt ? outcomeLabel(attempt.outcome) : "—"}
										</Table.Cell>
										<Table.Cell class="text-muted-foreground">
											{attempt?.statusCode ?? "—"}
										</Table.Cell>
										<Table.Cell class="text-muted-foreground">
											{attempt ? duration(attempt.elapsedMs) : "—"}
										</Table.Cell>
									</Table.Row>

									{#if open}
										<Table.Row>
											<Table.Cell colspan={logColumns} class="h-auto py-3 whitespace-normal">
												<div id={`delivery-${delivery.id}`} class="flex flex-col gap-3">
													<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
														<span class="text-sm text-ink-900">{deliverySummary(delivery)}</span>
														{#if delivery.subjectKind}
															<span class="text-xs text-muted-foreground">
																{delivery.subjectKind}
																{delivery.subjectId ?? ""}
															</span>
														{/if}
														{#if delivery.nextAttemptAt}
															<span class="text-xs text-muted-foreground">
																Next attempt {onDateAndTime(
																	delivery.nextAttemptAt,
																	workspace.timezone
																)}
															</span>
														{/if}
														<Button
															variant="secondary"
															size="sm"
															class="ml-auto"
															disabled={pending}
															onclick={() => replay(delivery.id)}
														>
															{replaying === delivery.id ? "Replaying…" : "Replay"}
														</Button>
													</div>

													{#if !details[delivery.id]}
														<p class="text-xs text-muted-foreground">Loading this delivery…</p>
													{:else}
														<ul class="flex flex-col gap-2">
															{#each details[delivery.id].attempts as tried (tried.attempt)}
																<li class="flex flex-col gap-1 rounded-md border border-line-subtle p-3">
																	<div class="flex flex-wrap items-baseline gap-x-2 gap-y-1">
																		<span class="text-sm text-ink-900">
																			Attempt {tried.attempt}
																		</span>
																		<span class="text-xs text-muted-foreground">
																			{outcomeLabel(tried.outcome)}
																			{#if tried.statusCode}· {tried.statusCode}{/if}
																			· {duration(tried.elapsedMs)}
																			· {onDateAndTime(tried.startedAt, workspace.timezone)}
																		</span>
																	</div>
																	<p class="font-mono text-xs break-all text-muted-foreground">
																		{tried.requestUrl}
																		{#if tried.resolvedAddress}→ {tried.resolvedAddress}{/if}
																	</p>
																	{#if tried.error}
																		<p class="text-xs text-destructive">{tried.error}</p>
																	{/if}
																	{#if tried.responseExcerpt}
																		<pre
																			class="overflow-x-auto rounded-md bg-paper-100 p-2 font-mono text-xs text-ink-900">{tried.responseExcerpt}</pre>
																	{/if}
																</li>
															{/each}
														</ul>

														<div class="flex flex-col gap-1">
															<span class="text-xs font-medium tracking-snug text-ink-900">
																Payload
															</span>
															<pre
																class="overflow-x-auto rounded-md bg-paper-100 p-3 font-mono text-xs text-ink-900">{JSON.stringify(
																	details[delivery.id].body,
																	null,
																	2
																)}</pre>
														</div>
													{/if}
												</div>
											</Table.Cell>
										</Table.Row>
									{/if}
								{/each}
							</Table.Body>
						</Table.Root>

						{#if live.nextCursor}
							<div>
								<Button variant="secondary" size="sm" disabled={pending} onclick={more}>
									{working ? "Loading…" : "Load more"}
								</Button>
							</div>
						{/if}
					{/if}
				</section>
			{/if}
		</div>
	</div>
</div>
