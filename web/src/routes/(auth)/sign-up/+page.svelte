<script lang="ts">
	import { page } from "$app/state";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import Info from "@lucide/svelte/icons/info";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import {
		minPasswordLength,
		personalEmailDomain,
		signUpSchema,
	} from "$lib/auth/sign-up-schema";
	import type { SignUpOutcome } from "$lib/auth/types";
	import { signUpPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? signUpPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let submitOutcome = $state<SignUpOutcome | null>(null);

	const auth = $derived({ ...data.auth, ...preview?.auth });
	const outcome = $derived<SignUpOutcome | null>(submitOutcome ?? preview?.outcome ?? null);

	const form = superForm(defaults(zod4(signUpSchema)), {
		SPA: true,
		validators: zod4Client(signUpSchema),
		resetForm: false,
		onUpdate: ({ form: submitted }) => {
			if (!submitted.valid) return;
			submitOutcome = {
				kind: "verification_sent",
				email: submitted.data.email,
				sentAt: null,
				expiresAt: null,
			};
		},
	});
	const { form: formData, enhance, submitting } = form;

	$effect(() => {
		const prefill = preview?.form;
		if (prefill) formData.update((current) => ({ ...current, ...prefill }), { taint: false });
	});

	const busy = $derived(preview?.busy || $submitting);
	const valid = $derived(signUpSchema.safeParse($formData).success);

	const verification = $derived(outcome?.kind === "verification_sent" ? outcome : null);
	const ssoDomain = $derived(outcome?.kind === "domain_uses_sso" ? outcome : null);
	const showForm = $derived(auth.signupsOpen && !verification && !ssoDomain);
	const personalEmail = $derived(personalEmailDomain.test($formData.email));
	const emailRejected = $derived(personalEmail || outcome?.kind === "email_taken");
	const passwordMismatch = $derived(
		$formData.passwordConfirm.length > 0 && $formData.password !== $formData.passwordConfirm
	);

	const title = $derived(
		verification
			? "Check your email"
			: !auth.signupsOpen
				? "Signups are closed here"
				: ssoDomain
					? `${ssoDomain.organization} uses single sign-on`
					: "Create your account"
	);

	const lede = $derived(
		verification
			? "A confirmation link is on its way. It works once and lasts an hour."
			: !auth.signupsOpen
				? "This instance does not create accounts from the sign-up form."
				: ssoDomain
					? "Your domain is already set up with an identity provider."
					: busy
						? "Checking the address and reserving your workspace."
						: "Free for up to 10 people. You name your workspace next."
	);

	const notice = $derived.by(() => {
		if (!auth.signupsOpen) {
			return {
				variant: "muted" as const,
				icon: CircleDashed,
				title: "Signups are closed on this instance",
				body: "Accounts are created by invitation, or by your identity provider.",
			};
		}
		if (ssoDomain) {
			return {
				variant: "muted" as const,
				icon: Info,
				title: `This domain signs in through ${ssoDomain.provider}`,
				body: `${ssoDomain.provider} creates your account the first time you sign in. There is no password to set.`,
			};
		}
		if (outcome?.kind === "email_taken") {
			return {
				variant: "warning" as const,
				icon: CircleAlert,
				title: "An account already exists for this email",
				body: "Sign in instead, or reset the password from the sign-in screen.",
			};
		}
		if (outcome?.kind === "delivery_failed") {
			return {
				variant: "destructive" as const,
				icon: TriangleAlert,
				title: "Could not create the account",
				body: "Email delivery is down, so the confirmation link cannot be sent. Nothing was created.",
			};
		}
		if (personalEmail) {
			return {
				variant: "muted" as const,
				icon: Info,
				title: "Norn needs a work email",
				body: "Personal addresses cannot start a workspace. Use the address your team uses.",
			};
		}
		return null;
	});

	const passwordRules = $derived.by(() => {
		const password = $formData.password;
		const confirm = $formData.passwordConfirm;
		const longEnough = password.length >= minPasswordLength;
		return [
			{
				met: longEnough,
				label:
					password.length > 0 && !longEnough
						? `At least ${minPasswordLength} characters — this one is ${password.length}`
						: `At least ${minPasswordLength} characters`,
			},
			{ met: longEnough && confirm.length > 0 && password === confirm, label: "Both fields match" },
			{ met: false, label: "Checked against known breaches on submit" },
		];
	});

	const action = $derived.by(() => {
		if (verification) return { label: "I have confirmed, continue", href: "/create-workspace" };
		if (!auth.signupsOpen) return { label: "Go to sign in", href: "/sign-in" };
		if (ssoDomain) return { label: `Continue with ${ssoDomain.provider}`, href: "/sso" };
		if (outcome?.kind === "email_taken") return { label: "Sign in instead", href: "/sign-in" };
		if (busy) return { label: "Creating account", href: null };
		if (outcome?.kind === "delivery_failed") return { label: "Try again", href: null };
		return { label: "Create account", href: null };
	});

	const actionDisabled = $derived(busy || (action.href === null && !busy && !valid));

	function startAgain() {
		submitOutcome = null;
		formData.update((current) => ({ ...current, password: "", passwordConfirm: "", terms: false }));
	}

	const note = $derived(
		verification
			? "The link opens the workspace step."
			: showForm && !outcome && !busy && !valid
				? "A workspace is created for you. You can invite people right after."
				: null
	);
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-98">
		<div class="flex flex-col gap-4 p-5 sm:p-6">
			<div class="flex flex-col gap-1.5">
				<h1 class="text-2xl font-medium tracking-title text-ink-900">{title}</h1>
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

			{#if verification}
				<div class="flex flex-col gap-3">
					<dl
						class="flex flex-col gap-1 rounded-lg border border-line-strong bg-paper-0 px-3 py-2.5"
					>
						{#each [["sent to", verification.email], ["sent at", verification.sentAt], ["expires", verification.expiresAt]].filter(([, value]) => value) as [key, value] (key)}
							<div class="flex gap-2 font-mono text-xs leading-normal">
								<dt class="w-18 flex-none text-muted-foreground">{key}</dt>
								<dd class="min-w-0 flex-1 break-all text-ink-600">{value}</dd>
							</div>
						{/each}
					</dl>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Nothing is created until you open the link. Wrong address?
						<button
							type="button"
							onclick={startAgain}
							class="text-link hover:text-link-hover hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
						>
							Start again
						</button>.
					</p>
				</div>
			{/if}

			{#if showForm}
				<form id="sign-up-form" method="POST" use:enhance class="flex flex-col gap-4">
					<Form.Field {form} name="name">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Your name</Form.Label>
								<Input
									{...props}
									autocomplete="name"
									placeholder="Rae Okafor"
									disabled={busy}
									bind:value={$formData.name}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="email">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Work email</Form.Label>
								<Input
									{...props}
									type="email"
									inputmode="email"
									autocomplete="email"
									autocapitalize="none"
									spellcheck="false"
									placeholder="you@company.com"
									disabled={busy}
									aria-invalid={emailRejected ? "true" : undefined}
									bind:value={$formData.email}
								/>
							{/snippet}
						</Form.Control>
						{#if !emailRejected}
							<Form.Description class="text-sm text-muted-foreground">
								Used for sign-in and notifications.
							</Form.Description>
						{/if}
						<Form.FieldErrors />
					</Form.Field>

					<div class="flex flex-col gap-2">
						<Form.Field {form} name="password">
							<Form.Control>
								{#snippet children({ props })}
									<Form.Label>Password</Form.Label>
									<Input
										{...props}
										type="password"
										autocomplete="new-password"
										disabled={busy}
										bind:value={$formData.password}
									/>
								{/snippet}
							</Form.Control>
						</Form.Field>
						<ul class="flex flex-col gap-1">
							{#each passwordRules as rule (rule.label)}
								{@const RuleIcon = rule.met ? CircleCheck : CircleDashed}
								<li
									class="flex items-center gap-2 text-sm {rule.met
										? 'text-ink-600'
										: 'text-muted-foreground'}"
								>
									<RuleIcon
										class="size-icon-row shrink-0 {rule.met ? 'text-success' : ''}"
										aria-hidden="true"
									/>
									{rule.label}
								</li>
							{/each}
						</ul>
					</div>

					<Form.Field {form} name="passwordConfirm">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Repeat password</Form.Label>
								<Input
									{...props}
									type="password"
									autocomplete="new-password"
									disabled={busy}
									aria-invalid={passwordMismatch ? "true" : undefined}
									bind:value={$formData.passwordConfirm}
								/>
							{/snippet}
						</Form.Control>
						{#if passwordMismatch}
							<p class="flex items-center gap-1.5 text-sm text-destructive" role="alert">
								<CircleAlert class="size-icon-row shrink-0" aria-hidden="true" />
								Passwords do not match.
							</p>
						{/if}
					</Form.Field>

					<Form.Field {form} name="terms">
						<Form.Control>
							{#snippet children({ props })}
								<div class="flex items-start gap-2">
									<Checkbox {...props} disabled={busy} bind:checked={$formData.terms} />
									<Form.Label
										class="font-sans text-sm leading-normal tracking-normal text-ink-600 normal-case text-pretty"
									>
										I agree to the
										<a href="/terms" class="text-link hover:text-link-hover hover:underline">
											terms of service
										</a>
										and the
										<a href="/privacy" class="text-link hover:text-link-hover hover:underline">
											privacy notice
										</a>.
									</Form.Label>
								</div>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>
				</form>
			{/if}

			<div class="flex flex-col gap-2">
				{#if action.href}
					<Button href={action.href} class="w-full">{action.label}</Button>
				{:else}
					<Button type="submit" form="sign-up-form" class="w-full" disabled={actionDisabled}>
						{action.label}
					</Button>
				{/if}

				{#if showForm && !outcome && auth.sso}
					<Button href="/sso" variant="outline" class="w-full">
						Continue with {auth.sso.name}
					</Button>
				{/if}

				{#if note}
					<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">
						{note}
					</p>
				{/if}
			</div>
		</div>
	</div>

	<div class="flex w-full max-w-98 flex-col items-center gap-2">
		<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">
			Already have an account?
			<a href="/sign-in" class="text-link hover:text-link-hover hover:underline">Sign in</a>
		</p>
		<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">
			Joining a team that already uses Norn?
			<a href="/accept-invitation" class="text-link hover:text-link-hover hover:underline">
				Open your invitation
			</a>
		</p>
		{#if auth.selfHosted && auth.instance}
			<p class="text-center font-mono text-xs break-all text-muted-foreground">{auth.instance}</p>
		{/if}
	</div>
</div>
