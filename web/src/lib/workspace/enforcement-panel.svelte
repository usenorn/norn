<script lang="ts">
	import { invalidateAll } from "$app/navigation";
	import Copy from "@lucide/svelte/icons/copy";
	import KeyRound from "@lucide/svelte/icons/key-round";
	import ShieldCheck from "@lucide/svelte/icons/shield-check";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { api } from "$lib/api";
	import {
		blockerCopy,
		enforcementProblem,
		type Enforcement,
		type RecoveryCodes,
	} from "$lib/workspace/sso";

	let {
		workspace,
		enforcement,
		codes,
		busy = false,
		onbusy,
	}: {
		workspace: { id: string; name: string };
		enforcement: Enforcement;
		codes: RecoveryCodes;
		busy?: boolean;
		onbusy: (working: boolean) => void;
	} = $props();

	let saved = $state<Enforcement | null>(null);
	let issued = $state<RecoveryCodes | null>(null);
	let failed = $state(false);
	let working = $state(false);
	let copied = $state(false);

	const policy = $derived<Enforcement>(saved ?? enforcement);
	const shown = $derived<RecoveryCodes>(issued ?? codes);
	const enforcing = $derived(policy.kind === "available" && policy.enforcing);
	const disabled = $derived(busy || working);

	async function set(next: "any" | "sso") {
		working = true;
		failed = false;
		onbusy(true);

		try {
			const { data, error } = await api.PUT("/workspaces/{workspaceId}/auth-policy", {
				params: { path: { workspaceId: workspace.id } },
				body: { enforcement: next },
			});

			if (error) {
				const blocker = enforcementProblem(error);
				saved = blocker ? { kind: "blocked", blocker } : policy;
				failed = !blocker;

				return;
			}

			if (!data) return;

			saved = { kind: "available", enforcing: data.policy.enforcement === "sso" };
			issued = data.recoveryCodes?.length
				? { kind: "issued", codes: data.recoveryCodes }
				: { kind: "none" };

			await invalidateAll();
		} catch {
			failed = true;
		} finally {
			working = false;
			onbusy(false);
		}
	}

	async function copyCodes() {
		if (shown.kind !== "issued") return;

		await navigator.clipboard.writeText(shown.codes.join("\n"));
		copied = true;
	}
</script>

<section class="flex flex-col gap-4">
	<div class="flex flex-col gap-1">
		<h2 class="text-md font-medium tracking-snug text-ink-900">Require single sign-on</h2>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			With this on, a Norn password no longer opens {workspace.name}. People stay signed in to
			any other workspace they belong to, and automation keeps working through API tokens.
		</p>
	</div>

	{#if failed}
		<Alert.Root variant="destructive">
			<TriangleAlert aria-hidden="true" />
			<Alert.Title>Something went wrong</Alert.Title>
			<Alert.Description>Nothing changed. Wait a moment and try again.</Alert.Description>
		</Alert.Root>
	{/if}

	{#if policy.kind === "blocked"}
		<Alert.Root variant="warning">
			<TriangleAlert aria-hidden="true" />
			<Alert.Title>Not yet</Alert.Title>
			<Alert.Description>{blockerCopy[policy.blocker]}</Alert.Description>
		</Alert.Root>
	{/if}

	{#if shown.kind === "issued"}
		<div class="flex flex-col gap-2 rounded-lg border border-line-strong bg-paper-1 p-3">
			<div class="flex items-baseline justify-between gap-2">
				<Eyebrow class="text-ink-600">Recovery codes — shown once</Eyebrow>
				<Button variant="secondary" size="sm" onclick={copyCodes}>
					<Copy aria-hidden="true" />
					{copied ? "Copied" : "Copy all"}
				</Button>
			</div>
			<ul class="grid grid-cols-1 gap-1 sm:grid-cols-2">
				{#each shown.codes as code (code)}
					<li class="font-mono text-sm break-all text-ink-900">{code}</li>
				{/each}
			</ul>
			<p class="text-sm leading-normal text-muted-foreground text-pretty">
				Keep these somewhere that does not depend on signing in. If your provider becomes
				permanently unavailable, one of these lifts the requirement so passwords work again.
				Each one can be used once.
			</p>
		</div>
	{/if}

	<div class="flex flex-col gap-1 rounded-lg border border-line-subtle p-3">
		<Eyebrow class="text-ink-600">Status</Eyebrow>
		{#if policy.kind === "loading"}
			<p class="text-md text-muted-foreground" aria-live="polite">Loading…</p>
		{:else if enforcing}
			<p class="flex items-center gap-1.5 text-md text-ink-900">
				<ShieldCheck class="size-icon-toolbar shrink-0" aria-hidden="true" />
				Everyone signs in through your provider.
			</p>
		{:else}
			<p class="flex items-center gap-1.5 text-md text-ink-900">
				<KeyRound class="size-icon-toolbar shrink-0 text-muted-foreground" aria-hidden="true" />
				A Norn password still works here.
			</p>
		{/if}
	</div>

	<div>
		{#if enforcing}
			<Button variant="secondary" disabled={disabled} onclick={() => set("any")}>
				{working ? "Saving" : "Allow passwords again"}
			</Button>
		{:else}
			<Button disabled={disabled} onclick={() => set("sso")}>
				{working ? "Saving" : "Require single sign-on"}
			</Button>
		{/if}
	</div>
</section>
