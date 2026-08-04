<script lang="ts">
	import { page } from "$app/state";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import Diagnostics from "$lib/components/norn/diagnostics.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import InstanceLine from "$lib/components/norn/instance-line.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { goto } from "$app/navigation";
	import { api } from "$lib/api";
	import { signInSchema } from "$lib/auth/sign-in-schema";
	import { signInFailure } from "$lib/auth/sign-in";
	import type { SignInFailure } from "$lib/auth/types";
	import { signInPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? signInPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let submitFailure = $state<SignInFailure | null>(null);

	const auth = $derived({ ...data.auth, ...preview?.auth });
	const failure = $derived<SignInFailure | null>(submitFailure ?? preview?.failure ?? null);

	const form = superForm(defaults(zod4(signInSchema)), {
		SPA: true,
		validators: zod4Client(signInSchema),
		resetForm: false,
		onUpdate: async ({ form: submitted }) => {
			if (!submitted.valid) return;

			submitFailure = null;

			try {
				const { error } = await api.POST("/auth/login", {
					body: { email: submitted.data.email, password: submitted.data.password },
				});

				if (error) {
					submitFailure = signInFailure(error);

					return;
				}

				await goto("/", { invalidateAll: true });
			} catch {
				submitFailure = { kind: "unavailable" };
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	const ssoOnly = $derived(Boolean(auth.sso) && !auth.password);
	const showPassword = $derived(auth.password);
	const showSso = $derived(Boolean(auth.sso));
	const showDivider = $derived(showPassword && showSso);

	const locked = $derived(failure?.kind === "account_locked" || failure?.kind === "rate_limited");
	const credentialError = $derived(
		failure?.kind === "invalid_credentials" ? "Email or password is wrong." : null
	);

	const lede = $derived(
		ssoOnly
			? `This instance signs in through ${auth.sso?.name}. Password sign-in is off.`
			: showSso
				? "Use your work email, or your identity provider."
				: "Use your work email."
	);

	const notice = $derived.by(() => {
		if (failure?.kind === "account_locked") {
			return {
				variant: "destructive" as const,
				icon: CircleX,
				title: "Account locked",
				body: `Ten failed sign-ins in a row. It unlocks at ${failure.unlocksAt}, or an admin can unlock it now.`,
			};
		}
		if (failure?.kind === "rate_limited") {
			return {
				variant: "warning" as const,
				icon: TriangleAlert,
				title: "Too many attempts from this address",
				body: "Wait two minutes. The limit is per address, so a shared office network can trip it for everyone.",
			};
		}
		if (failure?.kind === "unavailable") {
			return {
				variant: "destructive" as const,
				icon: CircleX,
				title: "Couldn't reach the server",
				body: "Check your connection and try again. Nothing about your account changed.",
			};
		}
		if (failure?.kind === "sso_unavailable") {
			return {
				variant: "destructive" as const,
				icon: CircleX,
				title: "Single sign-on isn't answering",
				body: `${auth.sso?.name} is configured but its metadata URL returns 404. Password sign-in still works while you fix it.`,
			};
		}
		if (!auth.signupsOpen) {
			return {
				variant: "muted" as const,
				icon: CircleDashed,
				title: "Signups are closed on this instance",
				body: "Accounts are created by invitation, or automatically by your identity provider.",
			};
		}
		return null;
	});

	const diagnostics = $derived(failure?.kind === "sso_unavailable" ? failure.diagnostics : null);

	const footer = $derived.by(() => {
		if (failure?.kind === "invalid_credentials") {
			return `${failure.attemptsLeft} tries left. After that the account locks for 15 minutes.`;
		}
		if (failure?.kind === "account_locked") {
			return "Locking is per account, not per device — signing in elsewhere won't help.";
		}
		if (failure?.kind === "rate_limited") return "Nothing is wrong with your account.";
		if (failure?.kind === "sso_unavailable") return "Members see a shorter message than this one.";
		if (ssoOnly) return "Only an admin can turn password sign-in back on.";
		if (!auth.signupsOpen) return "This instance creates accounts by invitation only.";
		return "New to Norn? Create an account and start a workspace.";
	});

	const showSignupLink = $derived(!ssoOnly && auth.signupsOpen);
	const showInviteLink = $derived(!ssoOnly && !auth.signupsOpen);
</script>

<svelte:head><title>Sign in · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-form">
		<div class="flex flex-col gap-4 p-5 sm:p-6">
			<div class="flex flex-col gap-1.5">
				<h1 class="text-2xl font-medium tracking-title text-ink-900">Sign in</h1>
				<p class="text-md leading-normal text-muted-foreground text-pretty">{lede}</p>
			</div>

			{#if notice}
				{@const NoticeIcon = notice.icon}
				<Alert.Root variant={notice.variant}>
					<NoticeIcon aria-hidden="true" />
					<Alert.Title>{notice.title}</Alert.Title>
					<Alert.Description>{notice.body}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if diagnostics}
				<Diagnostics
					label="Provider response"
					entries={diagnostics}
					keyWidth="w-16 sm:w-21"
				/>
			{/if}

			{#if showPassword}
				<form method="POST" use:enhance class="flex flex-col gap-4">
					<Form.Field {form} name="email">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Email</Form.Label>
								<Input
									{...props}
									type="email"
									inputmode="email"
									autocomplete="email"
									autocapitalize="none"
									spellcheck="false"
									placeholder="you@company.com"
									disabled={locked}
									aria-invalid={credentialError ? "true" : undefined}
									bind:value={$formData.email}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="password">
						<Form.Control>
							{#snippet children({ props })}
								<div class="flex flex-wrap items-baseline justify-between gap-x-2">
									<Form.Label>Password</Form.Label>
									<a
										href="/reset-password"
										class="text-sm text-link hover:text-link-hover hover:underline"
									>
										Forgot password?
									</a>
								</div>
								<Input
									{...props}
									type="password"
									autocomplete="current-password"
									disabled={locked}
									aria-invalid={credentialError ? "true" : undefined}
									bind:value={$formData.password}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
						{#if credentialError}
							<p class="flex items-center gap-1.5 text-sm text-destructive" role="alert">
								<CircleAlert class="size-icon-row shrink-0" aria-hidden="true" />
								{credentialError}
							</p>
						{/if}
					</Form.Field>

					<Form.Button class="w-full" disabled={locked || $submitting}>
						{$submitting ? "Signing in" : "Sign in"}
					</Form.Button>
				</form>
			{/if}

			{#if showDivider}
				<div class="flex items-center gap-2">
					<span class="h-px flex-1 bg-line-subtle"></span>
					<Eyebrow>or</Eyebrow>
					<span class="h-px flex-1 bg-line-subtle"></span>
				</div>
			{/if}

			{#if showSso && auth.sso}
				<div class="flex flex-col gap-2">
					<Button href="/sso" variant={ssoOnly ? "default" : "secondary"} class="w-full">
						Continue with {auth.sso.name}
					</Button>
					{#if ssoOnly}
						<p class="text-center text-sm leading-normal text-muted-foreground">
							Wrong workspace?
							<a href="/sign-in" class="text-link hover:text-link-hover hover:underline">
								Sign in somewhere else
							</a>
						</p>
					{/if}
				</div>
			{/if}
		</div>
	</div>

	<div class="flex w-full max-w-form flex-col items-center gap-2">
		<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">
			{footer}
		</p>
		{#if showSignupLink}
			<a href="/sign-up" class="text-sm text-link hover:text-link-hover hover:underline">
				Create an account
			</a>
		{/if}
		{#if showInviteLink}
			<a href="/accept-invitation" class="text-sm text-link hover:text-link-hover hover:underline">
				Have an invitation? Open it
			</a>
		{/if}
		<InstanceLine instance={auth} />
	</div>
</div>
