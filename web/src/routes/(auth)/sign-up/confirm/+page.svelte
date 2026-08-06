<script lang="ts">
	import { enhance } from "$app/forms";
	import { page } from "$app/state";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import InstanceLine from "$lib/components/norn/instance-line.svelte";
	import StepList, { type Step } from "$lib/components/norn/step-list.svelte";
	import type { SignUpConfirmation } from "$lib/auth/types";
	import { signUpConfirmPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data, form: submitted }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? signUpConfirmPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let heading = $state<HTMLHeadingElement | null>(null);
	let confirming = $state<HTMLFormElement | null>(null);

	const auth = $derived(data.auth);
	const confirmation = $derived<SignUpConfirmation>(
		submitted?.confirmation ?? preview ?? data.confirmation
	);

	let started = false;

	$effect(() => {
		if (preview || submitted || started || data.confirmation.kind !== "confirming") return;

		started = true;
		confirming?.requestSubmit();
	});

	$effect(() => {
		if (confirmation.kind === "confirming") return;
		heading?.focus();
	});

	const steps = $derived<Step[]>([
		{ label: "Link opened", state: "done" },
		{ label: "Creating your account", state: "active" },
		{ label: "Signing you in", state: "waiting" },
	]);

	const inProgress = $derived(
		confirmation.kind === "confirming" || confirmation.kind === "confirmed"
	);

	const title = $derived.by(() => {
		switch (confirmation.kind) {
			case "confirming":
				return "Confirming your email";
			case "confirmed":
				return "You're in";
			case "no_token":
				return "Open the link from your email";
			case "expired":
				return "This link expired";
			case "invalid":
				return "This link isn't recognised";
			case "used":
				return "This link was already used";
			case "email_taken":
				return "An account already exists for this email";
			case "unavailable":
				return "Something went wrong";
		}
	});

	const lede = $derived.by(() => {
		switch (confirmation.kind) {
			case "confirming":
				return "This takes a second. Your account is created as the link is opened.";
			case "confirmed":
				return "Your account is ready. Taking you to your workspace.";
			case "no_token":
				return "This page finishes a sign-up. Open the confirmation link we sent you.";
			case "expired":
				return "Confirmation links last an hour and this one is past it. Nothing was created.";
			case "invalid":
				return "The link is incomplete, or a newer one replaced it. Only the most recent link works.";
			case "used":
				return "Your account already exists. Sign in with the password you chose.";
			case "email_taken":
				return "Someone finished signing up with this address first. Sign in instead.";
			case "unavailable":
				return "We could not reach the server. The link has not been used, so you can try again.";
		}
	});

	const notice = $derived.by(() => {
		switch (confirmation.kind) {
			case "expired":
			case "invalid":
				return {
					variant: "warning" as const,
					icon: CircleAlert,
					title: "Start over to get a new link",
					body: "Signing up again sends a fresh link and cancels this one.",
				};
			case "used":
			case "email_taken":
				return {
					variant: "muted" as const,
					icon: CircleDashed,
					title: "Nothing more to confirm",
					body: "Reset your password from the sign-in screen if you cannot get in.",
				};
			case "unavailable":
				return {
					variant: "destructive" as const,
					icon: TriangleAlert,
					title: "Could not reach the server",
					body: "Check your connection. Your link is still unused.",
				};
			default:
				return null;
		}
	});

	const action = $derived.by(() => {
		switch (confirmation.kind) {
			case "confirmed":
				return { label: "Continue", href: "/", onclick: null };
			case "no_token":
				return { label: "Create an account", href: "/sign-up", onclick: null };
			case "expired":
			case "invalid":
				return { label: "Start again", href: "/sign-up", onclick: null };
			case "used":
			case "email_taken":
				return { label: "Sign in", href: "/sign-in", onclick: null };
			case "unavailable":
				return { label: "Try again", href: null, onclick: null };
			default:
				return null;
		}
	});
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-98">
		<div class="flex flex-col gap-4 p-5 sm:p-6">
			<div class="flex flex-col gap-1.5">
				{#if inProgress}
					<Eyebrow>Step 2 of 4</Eyebrow>
				{/if}
				<h1
					bind:this={heading}
					tabindex="-1"
					class="text-2xl font-medium tracking-title text-ink-900"
				>
					{title}
				</h1>
				<p class="text-md leading-normal text-muted-foreground text-pretty">{lede}</p>
			</div>

			{#if confirmation.kind === "confirming"}
				<div class="flex flex-col gap-2" aria-live="polite" aria-busy="true">
					<Progress indeterminate aria-label="Confirming your email" />
					<StepList {steps} />
				</div>
			{:else if confirmation.kind === "confirmed"}
				<div class="flex items-start gap-2 rounded-lg border border-line-strong bg-paper-0 px-3 py-2.5">
					<CircleCheck class="mt-px size-icon-row shrink-0 text-success" aria-hidden="true" />
					<p class="min-w-0 flex-1 font-mono text-xs break-all text-ink-600">
						{confirmation.email}
					</p>
				</div>
			{:else if notice}
				{@const NoticeIcon = notice.icon}
				<Alert.Root variant={notice.variant}>
					<NoticeIcon aria-hidden="true" />
					<Alert.Title>{notice.title}</Alert.Title>
					<Alert.Description>{notice.body}</Alert.Description>
				</Alert.Root>
			{:else if confirmation.kind === "no_token"}
				<Alert.Root variant="muted">
					<CircleX aria-hidden="true" />
					<Alert.Title>No confirmation token in this address</Alert.Title>
					<Alert.Description>
						Open the link from the email rather than typing this address by hand.
					</Alert.Description>
				</Alert.Root>
			{/if}

			<form bind:this={confirming} method="POST" use:enhance class="contents">
				<input type="hidden" name="token" value={data.token ?? ""} />

				{#if action}
					<div class="flex flex-col gap-2">
						{#if action.href}
							<Button href={action.href} class="w-full">{action.label}</Button>
						{:else}
							<Button type="submit" class="w-full">{action.label}</Button>
						{/if}
					</div>
				{:else if confirmation.kind === "confirming"}
					<noscript>
						<Button type="submit" class="w-full">Confirm your email</Button>
					</noscript>
				{/if}
			</form>
		</div>
	</div>

	<div class="flex w-full max-w-98 flex-col items-center gap-2">
		{#if confirmation.kind !== "confirmed"}
			<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">
				Already have an account?
				<a href="/sign-in" class="text-link hover:text-link-hover hover:underline">Sign in</a>
			</p>
		{/if}
		<InstanceLine instance={auth} />
	</div>
</div>
