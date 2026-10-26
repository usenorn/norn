<script lang="ts">
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import UserPlus from "@lucide/svelte/icons/user-plus";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "#lib/components/ui/alert/index.js";
	import * as Form from "#lib/components/ui/form/index.js";
	import Diagnostics from "#lib/components/norn/diagnostics.svelte";
	import Eyebrow from "#lib/components/norn/eyebrow.svelte";
	import InstanceLine from "#lib/components/norn/instance-line.svelte";
	import { Button } from "#lib/components/ui/button/index.js";
	import { Input } from "#lib/components/ui/input/index.js";
	import SignInProviders from "#lib/auth/sign-in-providers.svelte";
	import { identifySchema, signInSchema } from "#lib/auth/sign-in-schema.js";
	import { clockTime } from "#lib/auth/sign-in.js";
	import { methodGroups, type SignInOutcome } from "#lib/auth/sign-in-step.js";
	import { actionPath, authPath, safeReturn } from "#lib/auth/return-to.js";
	import { signInPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const identifyFormId = "sign-in-identify";
	const passwordFormId = "sign-in-password";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? signInPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	const returnTo = $derived(safeReturn(page.url.searchParams.get("return")));
	const carried = $derived({
		add: page.url.searchParams.get("add") ?? "",
		return: page.url.searchParams.get("return") ?? "",
	});

	const auth = $derived({ ...data.auth, ...preview?.auth });
	const entry = $derived(preview?.entry ?? data.entry);

	// svelte-ignore state_referenced_locally
	const identifyForm = superForm(data.identifyForm, {
		id: identifyFormId,
		validators: zod4Client(identifySchema),
		resetForm: false,
	});
	const {
		form: identifyData,
		enhance: identifyEnhance,
		submitting: identifying,
		message: identifyMessage,
	} = identifyForm;

	// svelte-ignore state_referenced_locally
	const passwordForm = superForm(data.passwordForm, {
		id: passwordFormId,
		validators: zod4Client(signInSchema),
		resetForm: false,
	});
	const {
		form: passwordData,
		enhance: passwordEnhance,
		submitting: signingIn,
		message: passwordMessage,
	} = passwordForm;

	const outcome = $derived<SignInOutcome>(
		$passwordMessage ?? $identifyMessage ?? preview?.outcome ?? { step: data.step }
	);
	const step = $derived(outcome.step);
	const failure = $derived(outcome.failure ?? null);

	$effect(() => {
		if (step.kind !== "methods" || !step.email) return;
		const { email } = step;
		passwordData.update((current) => ({ ...current, email }), { taint: false });
	});

	const groups = $derived(step.kind === "methods" ? methodGroups(step.options) : []);
	const offersPassword = $derived(groups.some((group) => group.method === "password"));
	const providers = $derived(groups.flatMap((group) => (group.method === "sso" ? group.workspaces : [])));
	const ssoOnly = $derived(providers.length > 0 && !offersPassword);
	const knownEmail = $derived(step.kind === "identify" ? "" : step.email);
	const workspaceName = $derived(entry.kind === "ready" ? entry.signIn.name : "");

	const locked = $derived(failure?.kind === "account_locked" || failure?.kind === "rate_limited");
	const busy = $derived($identifying || $signingIn);
	const credentialError = $derived(
		failure?.kind === "invalid_credentials" ? "Email or password is wrong." : null
	);

	const lede = $derived.by(() => {
		if (step.kind === "identify") return "Enter your email. We'll take you to the way your workspace signs in.";
		if (step.kind === "closed") return "Nothing on this instance accepts a sign-in from this address.";
		if (ssoOnly && providers.length > 1) {
			return "More than one workspace signs people in from your domain. Choose the one you want.";
		}
		if (ssoOnly) {
			return `${workspaceName || "This workspace"} signs in through ${providers[0].host ?? "single sign-on"}. Password sign-in is off for this workspace.`;
		}
		if (providers.length > 0) return "Use your identity provider, or your password.";
		return knownEmail ? "Enter your password to continue." : "Use the email you signed up with.";
	});

	const notice = $derived.by(() => {
		if (failure?.kind === "account_locked") {
			return {
				variant: "destructive" as const,
				icon: CircleX,
				title: "Account locked",
				body: `Ten failed sign-ins in a row. It unlocks at ${clockTime(failure.unlocksAt)}, or an admin can unlock it now.`,
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
				body: `${auth.sso?.name ?? "Your provider"} is configured but its metadata URL returns 404. Password sign-in still works while you fix it.`,
			};
		}
		if (step.kind === "closed") {
			return {
				variant: "muted" as const,
				icon: CircleDashed,
				title: "No way in from this address",
				body: "Password sign-in is off here and no workspace has verified your email domain. Ask your admin for an invitation link.",
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
		if (!auth.signupsOpen) {
			return "Invitations arrive by email. Open the link in yours to create an account.";
		}
		return "New to Norn? Create an account and start a workspace.";
	});

	const showSignupLink = $derived(!ssoOnly && auth.signupsOpen);
</script>

<svelte:head><title>Sign in · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-form">
		<div class="flex flex-col gap-4.5 p-6.5 pb-5.5">
			<div class="flex flex-col gap-1.5">
				<h1 class="text-2xl font-semibold tracking-title text-ink-900">Sign in</h1>
				<p class="text-md leading-normal text-muted-foreground text-pretty">{lede}</p>
			</div>

			{#if data.adding}
				<Alert.Root variant="muted">
					<UserPlus aria-hidden="true" />
					<Alert.Title>You're already signed in</Alert.Title>
					<Alert.Description>
						Signing in adds a second account to this browser. You can switch between them at any
						time, and sign out of either on its own.
					</Alert.Description>
					<Alert.Action>
						<Button href={data.cancelTo} variant="secondary" size="sm">Cancel</Button>
					</Alert.Action>
				</Alert.Root>
			{/if}

			{#if notice}
				{@const NoticeIcon = notice.icon}
				<Alert.Root variant={notice.variant}>
					<NoticeIcon aria-hidden="true" />
					<Alert.Title>{notice.title}</Alert.Title>
					<Alert.Description>{notice.body}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if entry.kind === "unknown"}
				<Alert.Root variant="muted">
					<CircleDashed aria-hidden="true" />
					<Alert.Title>No workspace at that address</Alert.Title>
					<Alert.Description>
						Check the address with whoever invited you. It is the last part of the link your team
						signs in at.
					</Alert.Description>
				</Alert.Root>
			{/if}

			{#if diagnostics}
				<Diagnostics label="Provider response" entries={diagnostics} keyWidth="w-16 sm:w-21" />
			{/if}

			{#if step.kind === "identify"}
				<form
					id={identifyFormId}
					method="POST"
					action={actionPath(page.url, "identify")}
					use:identifyEnhance
					class="flex flex-col gap-4"
				>
					<input type="hidden" name="add" value={carried.add} />
					<input type="hidden" name="return" value={carried.return} />

					<Form.Field form={identifyForm} name="email">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Email</Form.Label>
								<Input
									{...props}
									type="email"
									inputmode="email"
									autocomplete="username"
									autocapitalize="none"
									spellcheck="false"
									placeholder="you@example.com"
									disabled={busy || locked}
									bind:value={$identifyData.email}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Button class="w-full" disabled={busy || locked}>
						{$identifying ? "Checking" : "Continue"}
					</Form.Button>
				</form>

				<a
					href={authPath(page.url, "/sign-in/sso")}
					class="text-center text-sm text-link hover:text-link-hover hover:underline"
				>
					Sign in with your workspace address
				</a>
			{:else}
				{#if knownEmail}
					<p class="flex flex-wrap items-baseline justify-between gap-x-2 text-md text-ink-900">
						<span class="min-w-0 break-all">{knownEmail}</span>
						<a href={authPath(page.url, "/sign-in")} class="text-sm text-link hover:text-link-hover hover:underline">
							Not you?
						</a>
					</p>
				{/if}

				{#each groups as group, index (group.method)}
					{#if index > 0}
						<div class="flex items-center gap-2.5">
							<span class="h-px flex-1 bg-line-subtle"></span>
							<Eyebrow>or</Eyebrow>
							<span class="h-px flex-1 bg-line-subtle"></span>
						</div>
					{/if}

					{#if group.method === "sso"}
						<SignInProviders workspaces={group.workspaces} {returnTo} preferred={index === 0} />
					{:else}
						<form
							id={passwordFormId}
							method="POST"
							action={actionPath(page.url, "password")}
							use:passwordEnhance
							class="flex flex-col gap-4"
						>
							<input type="hidden" name="add" value={carried.add} />
							<input type="hidden" name="return" value={carried.return} />
							<input type="hidden" name="workspace" value={step.kind === "methods" ? step.workspace : ""} />

							{#if knownEmail}
								<input type="hidden" name="email" value={knownEmail} />
							{:else}
								<Form.Field form={passwordForm} name="email">
									<Form.Control>
										{#snippet children({ props })}
											<Form.Label>Email</Form.Label>
											<Input
												{...props}
												type="email"
												inputmode="email"
												autocomplete="username"
												autocapitalize="none"
												spellcheck="false"
												placeholder="you@example.com"
												disabled={busy || locked}
												aria-invalid={credentialError ? "true" : undefined}
												bind:value={$passwordData.email}
											/>
										{/snippet}
									</Form.Control>
									<Form.FieldErrors />
								</Form.Field>
							{/if}

							<Form.Field form={passwordForm} name="password">
								<Form.Control>
									{#snippet children({ props })}
										<div class="flex flex-wrap items-baseline justify-between gap-x-2">
											<Form.Label>Password</Form.Label>
											<a
												href={authPath(page.url, "/reset-password")}
												class="text-sm text-link hover:text-link-hover hover:underline"
											>
												Forgot password?
											</a>
										</div>
										<Input
											{...props}
											type="password"
											autocomplete="current-password"
											disabled={busy || locked}
											aria-invalid={credentialError ? "true" : undefined}
											bind:value={$passwordData.password}
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

							<Form.Button class="w-full" disabled={busy || locked}>
								{$signingIn ? "Signing in" : "Sign in"}
							</Form.Button>
						</form>
					{/if}
				{/each}

				{#if ssoOnly && !knownEmail}
					<p class="text-center text-sm leading-normal text-muted-foreground">
						Wrong workspace?
						<a href={authPath(page.url, "/sign-in")} class="text-link hover:text-link-hover hover:underline">
							Sign in somewhere else
						</a>
					</p>
				{/if}
			{/if}
		</div>
	</div>

	<div class="flex w-full max-w-form flex-col items-center gap-2">
		<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">
			{footer}
		</p>
		{#if showSignupLink}
			<a href={authPath(page.url, "/sign-up")} class="text-sm text-link hover:text-link-hover hover:underline">
				Create an account
			</a>
		{/if}
		<InstanceLine instance={auth} />
	</div>
</div>
