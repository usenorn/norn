<script lang="ts">
	import { invalidate } from "$app/navigation";
	import { page } from "$app/state";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import * as Form from "#lib/components/ui/form/index.js";
	import { Button } from "#lib/components/ui/button/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import CommandSnippet from "#lib/components/norn/command-snippet.svelte";
	import Eyebrow from "#lib/components/norn/eyebrow.svelte";
	import { api } from "#lib/api/index.js";
	import { keys } from "#lib/api/keys.js";
	import { onDate } from "#lib/time.js";
	import { domainSchema } from "./domain-schema";
	import {
		domainFailure,
		domainFailureCopy,
		domainStatus,
		type DomainFailure,
		type DomainList,
		type WorkspaceDomain,
	} from "./domains";

	let {
		workspace,
		domains,
		busy = false,
		onbusy,
	}: {
		workspace: { id: string; name: string; timezone: string };
		domains: DomainList;
		busy?: boolean;
		onbusy: (working: boolean) => void;
	} = $props();

	let working = $state<string | null>(null);
	let failure = $state<DomainFailure | null>(null);
	let copied = $state<string | null>(null);

	const disabled = $derived(busy || working !== null);

	async function settle(run: () => Promise<{ error?: Parameters<typeof domainFailure>[0] }>, key: string) {
		working = key;
		failure = null;
		onbusy(true);

		try {
			const { error } = await run();

			if (error) {
				failure = domainFailure(error);

				return false;
			}

			await invalidate(keys.page(page.route.id));

			return true;
		} catch {
			failure = "unavailable";

			return false;
		} finally {
			working = null;
			onbusy(false);
		}
	}

	const form = superForm(defaults(zod4(domainSchema)), {
		id: "workspace-domain-form",
		SPA: true,
		validators: zod4Client(domainSchema),
		resetForm: true,
		invalidateAll: false,
		onUpdate: async ({ form: pending, cancel }) => {
			if (!pending.valid) return;

			const added = await settle(
				() =>
					api.POST("/workspaces/{workspaceId}/sso/domains", {
						params: { path: { workspaceId: workspace.id } },
						body: { domain: pending.data.domain },
					}),
				"add"
			);

			if (!added) cancel();
		},
	});

	const { form: formData, enhance, submitting } = form;

	function verify(domain: WorkspaceDomain) {
		return settle(
			() =>
				api.POST("/workspaces/{workspaceId}/sso/domains/{domainId}/verify", {
					params: { path: { workspaceId: workspace.id, domainId: domain.id } },
				}),
			`verify:${domain.id}`
		);
	}

	function remove(domain: WorkspaceDomain) {
		return settle(
			() =>
				api.DELETE("/workspaces/{workspaceId}/sso/domains/{domainId}", {
					params: { path: { workspaceId: workspace.id, domainId: domain.id } },
				}),
			`remove:${domain.id}`
		);
	}

	async function copyRecord(domain: WorkspaceDomain) {
		await navigator.clipboard.writeText(domain.record);
		copied = domain.id;
	}
</script>

<section class="flex flex-col gap-4">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-semibold tracking-snug text-ink-900">Email domains</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			People whose email is at a verified domain go straight to {workspace.name}'s provider when
			they sign in. Prove you own a domain by publishing a DNS TXT record on it.
		</p>
	</div>

	{#if failure}
		<Alert.Root variant={failure === "unavailable" || failure === "lookup_failed" ? "destructive" : "warning"}>
			<TriangleAlert aria-hidden="true" />
			<Alert.Title>
				{failure === "unavailable" || failure === "lookup_failed" ? "Something went wrong" : "Not added"}
			</Alert.Title>
			<Alert.Description>{domainFailureCopy[failure]}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if domains.kind === "unavailable"}
		<Alert.Root variant="destructive">
			<TriangleAlert aria-hidden="true" />
			<Alert.Title>Domains couldn't be loaded</Alert.Title>
			<Alert.Description>Reload the page to try again.</Alert.Description>
		</Alert.Root>
	{:else if domains.domains.length === 0}
		<p class="text-sm leading-normal text-muted-foreground">No domains yet.</p>
	{:else}
		<ul class="flex flex-col gap-3">
			{#each domains.domains as domain (domain.id)}
				{@const status = domainStatus(domain)}
				<li class="flex flex-col gap-2 rounded-lg border border-line-subtle p-3">
					<div class="flex flex-wrap items-baseline justify-between gap-2">
						<p class="min-w-0 break-all text-md font-medium text-ink-900">{domain.domain}</p>
						<p class="text-sm text-muted-foreground">
							{#if status.kind === "verified"}
								Verified on {onDate(status.at, workspace.timezone)}
							{:else if status.kind === "not_found"}
								Record not found, checked {onDate(status.checkedAt, workspace.timezone)}
							{:else}
								Waiting for the record
							{/if}
						</p>
					</div>

					{#if status.kind !== "verified"}
						<div class="flex flex-col gap-1.5">
							<Eyebrow class="text-ink-600">TXT record on {domain.domain}</Eyebrow>
							<CommandSnippet
								command={domain.record}
								label="Copy record"
								copied={copied === domain.id}
								oncopy={() => copyRecord(domain)}
							/>
						</div>
					{/if}

					<div class="flex flex-wrap gap-2">
						{#if status.kind !== "verified"}
							<Button size="sm" {disabled} onclick={() => verify(domain)}>
								{working === `verify:${domain.id}` ? "Checking" : "Verify"}
							</Button>
						{/if}
						<Button variant="secondary" size="sm" {disabled} onclick={() => remove(domain)}>
							{working === `remove:${domain.id}` ? "Removing" : "Remove"}
						</Button>
					</div>
				</li>
			{/each}
		</ul>
	{/if}

	<form id="workspace-domain-form" method="POST" use:enhance class="flex flex-col gap-2 sm:flex-row sm:items-start">
		<Form.Field {form} name="domain" class="flex-1">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label class="sr-only">Domain</Form.Label>
					<Input
						{...props}
						autocapitalize="none"
						spellcheck="false"
						placeholder="northwind.co"
						disabled={disabled || $submitting}
						bind:value={$formData.domain}
					/>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>
		<Button type="submit" variant="secondary" disabled={disabled || $submitting}>
			{working === "add" ? "Adding" : "Add domain"}
		</Button>
	</form>
</section>
