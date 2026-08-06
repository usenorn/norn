<script lang="ts">
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Info from "@lucide/svelte/icons/info";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import InstanceLine from "$lib/components/norn/instance-line.svelte";
	import { api } from "$lib/api";
	import { newPasswordSchema, resetRequestSchema } from "$lib/auth/reset-password-schema";
	import {
		emailMessage,
		passwordMessage,
		resetLinkFailure,
		resetRequestFailure,
		resetSent,
	} from "$lib/auth/password-reset";
	import { minPasswordLength } from "$lib/auth/sign-up-schema";
	import type { PasswordReset } from "$lib/auth/types";
	import { resetPasswordPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const requestFormId = "reset-request-form";
	const passwordFormId = "new-password-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? resetPasswordPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let submitReset = $state<PasswordReset | null>(null);

	const auth = $derived({ ...data.auth, ...preview?.auth });
	const reset = $derived<PasswordReset>(submitReset ?? preview?.reset ?? data.reset);

	const requestForm = superForm(defaults(zod4(resetRequestSchema)), {
		id: requestFormId,
		SPA: true,
		validators: zod4Client(resetRequestSchema),
		resetForm: false,
		onUpdate: async ({ form: submitted }) => {
			if (!submitted.valid) return;

			try {
				const { data: accepted, error } = await api.POST("/auth/password-reset", {
					body: { email: submitted.data.email },
				});

				if (error) {
					const failure = resetRequestFailure(error);

					if (failure) {
						submitReset = failure;

						return;
					}

					for (const field of error.errors ?? []) {
						if (field.field === "email") {
							setError(submitted, "email", emailMessage(field.code));
						}
					}

					return;
				}

				submitReset = resetSent(submitted.data.email, accepted, new Date());
			} catch {
				submitReset = { kind: "unavailable" };
			}
		},
	});
	const { form: requestData, enhance: requestEnhance, submitting: requestSubmitting } = requestForm;

	const passwordForm = superForm(defaults(zod4(newPasswordSchema)), {
		id: passwordFormId,
		SPA: true,
		validators: zod4Client(newPasswordSchema),
		resetForm: false,
		onUpdate: async ({ form: submitted }) => {
			if (!submitted.valid) return;

			if (!data.token) {
				submitReset = { kind: "link_expired" };

				return;
			}

			try {
				const { error } = await api.POST("/auth/password-reset/confirm", {
					body: { token: data.token, password: submitted.data.password },
				});

				if (!error) {
					submitReset = { kind: "changed" };

					return;
				}

				const outcome = resetLinkFailure(error);

				if (outcome) {
					submitReset = outcome;

					return;
				}

				for (const field of error.errors ?? []) {
					if (field.field === "password") {
						setError(submitted, "password", passwordMessage(field.code));
					}
				}
			} catch {
				submitReset = { kind: "unavailable" };
			}
		},
	});
	const {
		form: passwordData,
		enhance: passwordEnhance,
		submitting: passwordSubmitting,
	} = passwordForm;

	$effect(() => {
		if (reset.kind !== "sent") return;
		const { email } = reset;
		requestData.update((current) => ({ ...current, email }), { taint: false });
	});

	const busy = $derived($requestSubmitting || $passwordSubmitting);

	const ssoOnly = $derived(Boolean(auth.sso) && !auth.password);
	const provider = $derived(auth.sso?.name ?? "your identity provider");

	const showRequestForm = $derived(
		!ssoOnly &&
			(reset.kind === "request" ||
				reset.kind === "sent" ||
				reset.kind === "link_expired" ||
				reset.kind === "link_used")
	);
	const showPasswordForm = $derived(!ssoOnly && reset.kind === "form");

	const title = $derived.by(() => {
		if (ssoOnly) return "Passwords aren't used here";
		switch (reset.kind) {
			case "request":
				return "Reset your password";
			case "sent":
				return "Check your email";
			case "form":
				return "Choose a new password";
			case "link_expired":
				return "This link expired";
			case "link_used":
				return "This link was already used";
			case "changed":
				return "Password changed";
			case "mail_unavailable":
				return "This instance can't send email";
			case "unavailable":
				return "Something went wrong";
		}
	});

	const lede = $derived.by(() => {
		if (ssoOnly) {
			return `This instance signs in through ${provider}, so there's nothing here to reset.`;
		}
		switch (reset.kind) {
			case "request":
				return "We'll email a link that works once and lasts an hour.";
			case "sent":
				return `If ${reset.email} has an account here, the link is on its way.`;
			case "form":
				return "Setting a new password signs you out on every other device.";
			case "link_expired":
				return "Links last an hour. Ask for a fresh one and use it straight away.";
			case "link_used":
				return "Each link works once. If you didn't reset your password, tell an admin.";
			case "changed":
				return null;
			case "mail_unavailable":
				return "Password recovery needs email. Ask an admin to configure it, or to reset your password for you.";
			case "unavailable":
				return "We couldn't finish that just now. Wait a moment and try again.";
		}
	});

	const notice = $derived.by(() => {
		if (ssoOnly) {
			return {
				variant: "muted" as const,
				icon: Info,
				title: `${provider} owns your password`,
				body: "Reset it with your identity provider and it works here immediately.",
			};
		}
		if (reset.kind === "changed") {
			return {
				variant: "success" as const,
				icon: CircleCheck,
				title: "Signed out everywhere else",
				body: "Sessions on your other devices ended just now.",
			};
		}
		if (reset.kind === "mail_unavailable") {
			return {
				variant: "destructive" as const,
				icon: CircleX,
				title: "Email delivery isn't configured",
				body: "No link was sent. This instance has no mail server set up.",
			};
		}
		return null;
	});

	const passwordRules = $derived([
		{
			met: $passwordData.password.length >= minPasswordLength,
			label: `At least ${minPasswordLength} characters`,
		},
		{ met: false, label: "Not a password you've used here" },
		...(auth.breachCheck
			? [{ met: false, label: "Checked against known breaches on submit" }]
			: []),
	]);

	const action = $derived.by(() => {
		if (ssoOnly) return { label: `Continue with ${provider}`, href: "/sso", form: null };
		switch (reset.kind) {
			case "request":
				return { label: busy ? "Sending link" : "Send reset link", href: null, form: requestFormId };
			case "sent":
				return { label: busy ? "Sending link" : "Resend link", href: null, form: requestFormId };
			case "link_expired":
			case "link_used":
				return { label: busy ? "Sending link" : "Send a new link", href: null, form: requestFormId };
			case "form":
				return {
					label: busy ? "Setting password" : "Set password",
					href: null,
					form: passwordFormId,
				};
			case "changed":
				return { label: "Sign in", href: "/sign-in", form: null };
			case "mail_unavailable":
			case "unavailable":
				return { label: "Back to sign in", href: "/sign-in", form: null };
		}
	});

	const footer = $derived(
		reset.kind === "sent" ? "Nothing about your account changes until you open the link." : null
	);
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-form">
		<div class="flex flex-col gap-4.5 p-6.5 pb-5.5">
			<div class="flex flex-col gap-1.5">
				<h1 class="text-2xl font-medium tracking-title text-ink-900">{title}</h1>
				{#if lede}
					<p class="text-md leading-normal text-muted-foreground text-pretty">{lede}</p>
				{/if}
			</div>

			{#if notice}
				{@const NoticeIcon = notice.icon}
				<Alert.Root variant={notice.variant}>
					<NoticeIcon aria-hidden="true" />
					<Alert.Title>{notice.title}</Alert.Title>
					<Alert.Description>{notice.body}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if showRequestForm}
				<form id={requestFormId} method="POST" use:requestEnhance>
					{#if reset.kind === "sent"}
						<div class="flex flex-col gap-2.5 rounded-lg border border-line-default bg-paper-1 p-3">
							<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
								<span class="font-mono text-xs break-all text-ink-900">{reset.email}</span>
								<span class="font-mono text-xs text-muted-foreground">
									expires in {reset.expiresIn}
								</span>
							</div>
							<p class="text-sm leading-normal text-muted-foreground text-pretty">
								Check spam before asking an admin. The link works once and only on this instance.
							</p>
						</div>
					{:else}
						<Form.Field form={requestForm} name="email">
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
										disabled={busy}
										bind:value={$requestData.email}
									/>
								{/snippet}
							</Form.Control>
							<Form.FieldErrors />
						</Form.Field>
					{/if}
				</form>
			{/if}

			{#if showPasswordForm}
				<form id={passwordFormId} method="POST" use:passwordEnhance class="flex flex-col gap-4">
					<Form.Field form={passwordForm} name="password">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>New password</Form.Label>
								<Input
									{...props}
									type="password"
									autocomplete="new-password"
									disabled={busy}
									bind:value={$passwordData.password}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>
					<ul class="flex flex-col gap-1.5">
						{#each passwordRules as rule (rule.label)}
							{@const RuleIcon = rule.met ? CircleCheck : CircleDashed}
							<li
								class="flex items-center gap-2 font-mono text-xs {rule.met
									? 'text-ink-600'
									: 'text-muted-foreground'}"
							>
								<RuleIcon
									class="size-icon-row shrink-0 {rule.met ? 'text-success' : 'text-muted-foreground'}"
									aria-hidden="true"
								/>
								{rule.label}
							</li>
						{/each}
					</ul>
				</form>
			{/if}

			<div class="flex flex-col gap-2.5">
				{#if action.href}
					<Button href={action.href} class="w-full">{action.label}</Button>
				{:else}
					<Button type="submit" form={action.form} class="w-full" disabled={busy}>
						{action.label}
					</Button>
				{/if}
				{#if action.href !== "/sign-in"}
					<Button href="/sign-in" variant="ghost" class="w-full">Back to sign in</Button>
				{/if}
			</div>
		</div>
	</div>

	<div class="flex w-full max-w-form flex-col items-center gap-2">
		{#if footer}
			<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">{footer}</p>
		{/if}
		<InstanceLine instance={auth} />
	</div>
</div>
