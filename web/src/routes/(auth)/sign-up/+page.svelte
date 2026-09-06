<script lang="ts">
	import { page } from "$app/state";
	import { enhance as resendEnhance } from "$app/forms";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import UserPlus from "@lucide/svelte/icons/user-plus";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import Info from "@lucide/svelte/icons/info";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import { authPath } from "$lib/auth/return-to";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import InstanceLine from "$lib/components/norn/instance-line.svelte";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { minPasswordLength, signUpSchema } from "$lib/auth/sign-up-schema";
	import type { SignUpOutcome } from "$lib/auth/types";
	import { atClock } from "$lib/time";
	import { signUpPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV ? signUpPreviewStates[page.url.searchParams.get("state") ?? ""] : undefined
	);

	let heading = $state<HTMLHeadingElement | null>(null);
	let sent = $state<SignUpOutcome | null>(null);
	let timezone = $state("UTC");
	let secret = $state("");

	const auth = $derived({ ...data.auth, ...preview?.auth });

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		validators: zod4Client(signUpSchema),
		resetForm: false,
		onSubmit: () => {
			secret = $formData.password;
		},
	});
	const { form: formData, enhance, submitting, message } = form;

	$effect(() => {
		timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
	});

	$effect(() => {
		if ($message?.outcome) sent = $message.outcome;
	});

	const outcome = $derived<SignUpOutcome | null>(sent ?? preview?.outcome ?? null);
	const resend = $derived($message?.resend ?? preview?.resend ?? "idle");

	$effect(() => {
		const prefill = preview?.form;
		if (prefill) formData.update((current) => ({ ...current, ...prefill }), { taint: false });
	});

	const busy = $derived(Boolean(preview?.busy) || $submitting);
	const valid = $derived(signUpSchema.safeParse($formData).success);

	const verification = $derived(outcome?.kind === "verification_sent" ? outcome : null);
	const ssoDomain = $derived(outcome?.kind === "domain_uses_sso" ? outcome : null);
	const closed = $derived(!auth.signupsOpen || outcome?.kind === "closed");
	const showForm = $derived(!closed && !verification && !ssoDomain);
	const passwordMismatch = $derived(
		$formData.passwordConfirm.length > 0 && $formData.password !== $formData.passwordConfirm
	);

	$effect(() => {
		if (!outcome) return;
		heading?.focus();
	});

	const eyebrow = $derived(
		closed || ssoDomain ? null : verification ? "Step 2 of 4" : "Step 1 of 4"
	);

	const title = $derived(
		verification
			? "Check your email"
			: closed
				? "Signups are closed here"
				: ssoDomain
					? `${ssoDomain.organization} uses single sign-on`
					: "Create your account"
	);

	const lede = $derived(
		verification
			? "A confirmation link is on its way. It works once and lasts an hour."
			: closed
				? "This instance does not create accounts from the sign-up form."
				: ssoDomain
					? "Your domain is already set up with an identity provider."
					: busy
						? "Checking the address and reserving your workspace."
						: "Free for up to 10 people. You name your workspace next."
	);

	const notice = $derived.by(() => {
		if (closed) {
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
		if (outcome?.kind === "undeliverable") {
			return {
				variant: "destructive" as const,
				icon: TriangleAlert,
				title: "Could not create the account",
				body: "Email delivery is down, so the confirmation link cannot be sent. Nothing was created.",
			};
		}
		if (outcome?.kind === "rate_limited") {
			return {
				variant: "warning" as const,
				icon: CircleAlert,
				title: "Too many attempts from this address",
				body: "Wait a minute and try again.",
			};
		}
		if (outcome?.kind === "breach_check_unavailable") {
			return {
				variant: "warning" as const,
				icon: CircleAlert,
				title: "The password breach check is unavailable",
				body: "Passwords are checked against known breaches before an account is made. Try again shortly.",
			};
		}
		if (outcome?.kind === "refused") {
			return {
				variant: "destructive" as const,
				icon: TriangleAlert,
				title: "Could not create the account",
				body:
					outcome.reason ??
					"The server refused the request without saying why. Nothing was created. Try again, and tell us if it keeps happening.",
			};
		}
		if (outcome?.kind === "unavailable") {
			return {
				variant: "destructive" as const,
				icon: TriangleAlert,
				title: "Could not reach the server",
				body: "Nothing was created. Check your connection and try again.",
			};
		}
		return null;
	});

	const facts = $derived(
		verification
			? [
					["sent to", verification.email],
					["sent at", `${atClock(verification.requestedAt, "UTC")} UTC`],
					["expires", `${atClock(verification.expiresAt, "UTC")} UTC · one use`],
				]
			: []
	);

	const passwordRules = $derived.by(() => {
		const password = $formData.password;
		const confirm = $formData.passwordConfirm;
		const longEnough = password.length >= minPasswordLength;
		const rules = [
			{
				met: longEnough,
				label:
					password.length > 0 && !longEnough
						? `At least ${minPasswordLength} characters — this one is ${password.length}`
						: `At least ${minPasswordLength} characters`,
			},
			{ met: longEnough && confirm.length > 0 && password === confirm, label: "Both fields match" },
		];

		if (auth.breachCheck) {
			rules.push({ met: false, label: "Checked against known breaches on submit" });
		}

		return rules;
	});

	const action = $derived.by(() => {
		if (closed) return { label: "Go to sign in", href: "/sign-in", onclick: null };
		if (ssoDomain) {
			return { label: `Continue with ${ssoDomain.provider}`, href: "/sso", onclick: null };
		}
		if (outcome?.kind === "email_taken") {
			return { label: "Sign in instead", href: "/sign-in", onclick: null };
		}
		if (verification) {
			return { label: "I have confirmed, continue", href: "/create-workspace", onclick: null };
		}
		if (outcome?.kind === "undeliverable") {
			return { label: "Try again", href: null, onclick: null };
		}
		return {
			label: busy ? "Creating account" : "Create account",
			href: null,
			onclick: null,
		};
	});

	const actionDisabled = $derived(
		busy || (action.href === null && action.onclick === null && !valid)
	);

	function startAgain() {
		sent = null;
		formData.update((current) => ({ ...current, password: "", passwordConfirm: "", terms: false }));
	}

	const note = $derived(
		verification
			? "The link opens the workspace step."
			: showForm && !notice && !busy && !valid
				? "A workspace is created for you. You can invite people right after."
				: null
	);
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-98">
		<div class="flex flex-col gap-4.5 p-6.5 pb-5.5">
			<div class="flex flex-col gap-1.5">
				{#if eyebrow}
					<Eyebrow>{eyebrow}</Eyebrow>
				{/if}
				<h1
					bind:this={heading}
					tabindex="-1"
					class="text-2xl font-medium tracking-title text-ink-900 focus:outline-none"
				>
					{title}
				</h1>
				<p class="text-md leading-normal text-muted-foreground text-pretty">{lede}</p>
			</div>

			{#if data.adding}
				<Alert.Root variant="muted">
					<UserPlus aria-hidden="true" />
					<Alert.Title>You're already signed in</Alert.Title>
					<Alert.Description>
						Signing up creates a second account on this browser. You can switch between them at
						any time, and sign out of either on its own.
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

			{#if verification}
				<div class="flex flex-col gap-3">
					<dl class="flex flex-col gap-1 rounded-lg border border-line-strong bg-paper-0 px-3 py-2.5">
						{#each facts as [key, value] (key)}
							<div class="flex gap-2 font-mono text-xs leading-normal">
								<dt class="w-18 flex-none text-muted-foreground">{key}</dt>
								<dd class="min-w-0 flex-1 break-all text-ink-600">{value}</dd>
							</div>
						{/each}
					</dl>
					<div aria-live="polite">
						{#if resend === "limited"}
							<Alert.Root variant="warning">
								<CircleAlert aria-hidden="true" />
								<Alert.Title>Too many links requested</Alert.Title>
								<Alert.Description>
									Wait a minute before asking for another. The last link still works.
								</Alert.Description>
							</Alert.Root>
						{:else if resend === "unavailable"}
							<Alert.Root variant="destructive">
								<TriangleAlert aria-hidden="true" />
								<Alert.Title>Could not send another link</Alert.Title>
								<Alert.Description>
									The last link still works. Try again in a moment.
								</Alert.Description>
							</Alert.Root>
						{/if}
					</div>
					<form method="POST" action="?/resend" use:resendEnhance>
						<input type="hidden" name="name" value={$formData.name} />
						<input type="hidden" name="email" value={$formData.email} />
						<input type="hidden" name="password" value={secret} />
						<input type="hidden" name="passwordConfirm" value={secret} />
						<input type="hidden" name="terms" value="true" />
						<input type="hidden" name="timezone" value={timezone} />
						<p class="text-sm leading-normal text-muted-foreground text-pretty">
							Nothing is created until you open the link. Wrong address?
							<button
								type="button"
								onclick={startAgain}
								class="text-link hover:text-link-hover hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
							>
								Start again
							</button>. Nothing arrived?
							<button
								type="submit"
								disabled={busy}
								class="text-link hover:text-link-hover hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-40"
							>
								{busy ? "Sending" : "Send another link"}
							</button>.
						</p>
					</form>
				</div>
			{/if}

			{#if showForm}
				<form
					id="sign-up-form"
					method="POST"
					action="?/signUp"
					use:enhance
					class="flex flex-col gap-4"
				>
					<input type="hidden" name="timezone" value={timezone} />
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
								<Form.Label>Email</Form.Label>
								<Input
									{...props}
									type="email"
									inputmode="email"
									autocomplete="email"
									autocapitalize="none"
									spellcheck="false"
									placeholder="you@example.com"
									disabled={busy}
									bind:value={$formData.email}
								/>
							{/snippet}
						</Form.Control>
						<Form.Description class="text-sm text-muted-foreground">
							Used for sign-in and notifications.
						</Form.Description>
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
							<Form.FieldErrors />
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
								<label
									class="flex cursor-pointer items-start gap-2.25 text-sm leading-normal text-ink-600 text-pretty"
								>
									<Checkbox
										{...props}
										bind:checked={$formData.terms}
										disabled={busy}
										class="mt-px flex-none"
									/>
									<span>
										I agree to the
										<a href="/terms" class="text-link hover:text-link-hover hover:underline">
											terms of service
										</a>
										and the
										<a href="/privacy" class="text-link hover:text-link-hover hover:underline">
											privacy notice
										</a>.
									</span>
								</label>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>
				</form>
			{/if}

			<div class="flex flex-col gap-2.5">
				{#if action.href}
					<Button href={action.href} class="w-full">{action.label}</Button>
				{:else if action.onclick}
					<Button class="w-full" disabled={busy} onclick={action.onclick}>{action.label}</Button>
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
			<a href={authPath(page.url, "/sign-in")} class="text-link hover:text-link-hover hover:underline">Sign in</a>
		</p>
		<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">
			Joining a team that already uses Norn?
			<a href="/accept-invitation" class="text-link hover:text-link-hover hover:underline">
				Open your invitation
			</a>
		</p>
		<InstanceLine instance={auth} />
	</div>
</div>
