<script lang="ts">
	import { onMount } from "svelte";
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import Diagnostics from "$lib/components/norn/diagnostics.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import StepList, { type Step } from "$lib/components/norn/step-list.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import type { SsoExchange } from "$lib/auth/types";
	import { ssoPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? ssoPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	const startedExchange: SsoExchange = { status: "pending", phase: "redirecting" };

	const auth = $derived({ ...data.auth, ...preview?.auth });
	const exchange = $derived<SsoExchange>(preview?.exchange ?? startedExchange);
	const provider = $derived(auth.sso?.name ?? "your identity provider");

	let elapsedMs = $state(0);

	onMount(() => {
		const startedAt = performance.now();
		const tick = setInterval(() => (elapsedMs = performance.now() - startedAt), 100);
		return () => clearInterval(tick);
	});

	const elapsed = $derived(`${(elapsedMs / 1000).toFixed(1)}s`);

	const returning = $derived(exchange.status === "pending" && exchange.phase === "returning");

	const stage = $derived(returning ? "Signing in" : "Redirecting");
	const line = $derived(
		returning
			? `${provider} sent you back. We're checking the response and starting your session.`
			: `Taking you to ${provider}. You'll come back here automatically.`
	);
	const escape = $derived(returning ? "Cancel" : "Cancel and use a password");

	const steps = $derived<Step[]>(
		returning
			? [
					{ label: "Response received", state: "done" },
					{ label: "Signature verified", state: "done" },
					{ label: "Starting session", state: "active" },
				]
			: [
					{ label: "Request signed", state: "done" },
					{ label: `Handing off to ${provider}`, state: "active" },
					{ label: "Waiting for response", state: "waiting" },
				]
	);

	const report = $derived.by(() => {
		if (exchange.status !== "failed") return null;
		const { failure } = exchange;
		if (failure.kind === "rejected") {
			return {
				icon: CircleX,
				tone: "text-destructive",
				title: `${provider} wouldn't let you in`,
				body: "The provider rejected the sign-in before it reached us. Nothing is wrong with your Norn account.",
				fixTitle: "What to check",
				fixes: [
					`Are you assigned to the Norn app in ${provider}?`,
					`Does one of your ${provider} groups match a group mapped to this workspace?`,
					`If you're signed into two ${provider} accounts, sign out of the other one first.`,
				],
				primary: "Try again",
				secondary: "Sign in with password",
				diagnostics: failure.diagnostics,
				reference: failure.reference,
			};
		}
		if (failure.kind === "no_account") {
			return {
				icon: CircleAlert,
				tone: "text-warning",
				title: "Signed in, but there's no account here",
				body: `${provider} says you're ${failure.subject}. This workspace doesn't create accounts automatically, so someone has to invite you first.`,
				fixTitle: "What happens next",
				fixes: [
					`An admin invites ${failure.subject} and you sign in again.`,
					"Or an admin turns on just-in-time provisioning in Settings → Authentication.",
					"Nothing was created. You can close this tab safely.",
				],
				primary: "Request access",
				secondary: "Use another account",
				diagnostics: failure.diagnostics,
				reference: failure.reference,
			};
		}
		return {
			icon: TriangleAlert,
			tone: "text-destructive",
			title: `Couldn't reach ${provider}`,
			body: `The request timed out after ${failure.timeout}. This is between this instance and ${provider} — your account is fine.`,
			fixTitle: "What to check",
			fixes: [
				`Is ${provider} up? Check status.okta.com.`,
				"If this instance sits behind a proxy, confirm the proxy can reach okta.com.",
				"Password sign-in still works if an admin left it on.",
			],
			primary: "Try again",
			secondary: "Sign in with password",
			diagnostics: failure.diagnostics,
			reference: failure.reference,
		};
	});
</script>

<svelte:head><title>{report ? report.title : stage} · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	{#if !report}
		<div class="flex w-full max-w-90 flex-col items-center gap-6">
			<div class="flex w-full flex-col gap-3" aria-live="polite">
				<div class="flex items-baseline justify-between gap-2">
					<Eyebrow class="text-ink-600">{stage}</Eyebrow>
					<span class="font-mono text-xs text-muted-foreground tabular-nums">{elapsed}</span>
				</div>
				<Progress indeterminate aria-label={stage} />
				<p class="text-md leading-normal text-ink-600 text-pretty">{line}</p>
			</div>

			<StepList {steps} class="w-full" />

			<Button href="/sign-in" variant="ghost" size="sm">{escape}</Button>
		</div>
	{:else}
		{@const ReportIcon = report.icon}
		<div class="flex w-full max-w-115 flex-col gap-3">
			<div class="notch w-full">
				<div class="flex flex-col gap-4 p-5 sm:p-6">
					<div class="flex items-start gap-2">
						<ReportIcon
							class="mt-0.5 size-icon-toolbar shrink-0 {report.tone}"
							aria-hidden="true"
						/>
						<div class="flex flex-col gap-1.5">
							<h1 class="text-xl font-medium tracking-title text-ink-900">{report.title}</h1>
							<p class="text-md leading-normal text-ink-600 text-pretty">{report.body}</p>
						</div>
					</div>

					<Diagnostics label="Details" entries={report.diagnostics} />

					<div class="flex flex-col gap-2">
						<Eyebrow class="text-ink-600">{report.fixTitle}</Eyebrow>
						<ul class="flex flex-col gap-2">
							{#each report.fixes as fix (fix)}
								<li class="flex items-start gap-2">
									<span class="mt-2 size-1 flex-none bg-muted-foreground" aria-hidden="true"></span>
									<span class="flex-1 text-md leading-normal text-ink-600">{fix}</span>
								</li>
							{/each}
						</ul>
					</div>

					<div class="flex flex-wrap gap-2">
						<Button href="/sso">{report.primary}</Button>
						<Button href="/sign-in" variant="secondary">{report.secondary}</Button>
					</div>
				</div>
			</div>
			<p class="text-center font-mono text-xs break-all text-muted-foreground">
				{report.reference}
			</p>
		</div>
	{/if}
</div>
