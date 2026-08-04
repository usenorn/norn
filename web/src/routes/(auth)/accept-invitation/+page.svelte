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
	import { passwordMessage } from "$lib/auth/password-reset";
	import { minPasswordLength } from "$lib/auth/sign-up-schema";
	import { acceptInvitationSchema } from "$lib/workspace/accept-invitation-schema";
	import {
		linkFailure,
		type AcceptInvitation,
		type InvitationContext,
	} from "$lib/workspace/accept-invitation";
	import { roleLabels } from "$lib/workspace/members";
	import { acceptInvitationPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "accept-invitation-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? acceptInvitationPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let submitted = $state<AcceptInvitation | null>(null);
	let joining = $state(false);

	const auth = $derived(data.auth);
	const invitation = $derived<AcceptInvitation>(submitted ?? preview ?? data.invitation);

	const context = $derived<InvitationContext | undefined>(
		"workspace" in invitation && "email" in invitation
			? { workspace: invitation.workspace, email: invitation.email }
			: undefined
	);

	async function join(displayName?: string, password?: string) {
		if (!data.token) {
			submitted = { kind: "invalid" };

			return null;
		}

		const { data: accepted, error } = await api.POST("/invitations/accept", {
			body: {
				token: data.token,
				displayName,
				password,
				timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
			},
		});

		if (error) return error;

		if (accepted) {
			submitted = {
				kind: "joined",
				workspace: { slug: accepted.workspace.slug, name: accepted.workspace.name },
			};
		}

		return null;
	}

	const form = superForm(defaults(zod4(acceptInvitationSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(acceptInvitationSchema),
		resetForm: false,
		onUpdate: async ({ form: pending }) => {
			if (!pending.valid) return;

			try {
				const error = await join(pending.data.name, pending.data.password);

				if (!error) return;

				const failure = linkFailure(error, context);

				if (failure.kind !== "unavailable" || !error.errors?.length) {
					submitted = failure;

					return;
				}

				for (const field of error.errors) {
					if (field.field === "password") {
						setError(pending, "password", passwordMessage(field.code));
					}

					if (field.field === "display_name") {
						setError(pending, "name", "Enter your name.");
					}
				}
			} catch {
				submitted = { kind: "unavailable" };
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	async function confirmJoin() {
		joining = true;

		try {
			const error = await join();

			if (error) submitted = linkFailure(error, context);
		} catch {
			submitted = { kind: "unavailable" };
		} finally {
			joining = false;
		}
	}

	const busy = $derived($submitting || joining);

	const workspaceName = $derived(
		"workspace" in invitation ? invitation.workspace.name : ""
	);

	const title = $derived.by(() => {
		switch (invitation.kind) {
			case "no_token":
				return "Open your invitation link";
			case "invalid":
				return "This invitation link isn't recognised";
			case "expired":
				return "This invitation expired";
			case "revoked":
				return "This invitation was withdrawn";
			case "already_accepted":
				return "This invitation was already used";
			case "create_account":
			case "sign_in_required":
			case "confirm":
				return `Join ${invitation.workspace.name}`;
			case "address_mismatch":
				return "This invitation is for someone else";
			case "sso_required":
				return `${invitation.workspace.name} signs in through single sign-on`;
			case "joined":
				return "You're in";
			case "unavailable":
				return "Something went wrong";
		}
	});

	const lede = $derived.by(() => {
		switch (invitation.kind) {
			case "no_token":
				return "Invitations arrive by email. Open the link in that message, or paste the whole link into your address bar.";
			case "invalid":
				return "Check you copied the whole link. If it still doesn't work, ask an admin to send a new one.";
			case "expired":
				return "Links last seven days. Ask an admin to send a fresh one and use it straight away.";
			case "revoked":
				return "An administrator withdrew it before it was used. Ask them to invite you again.";
			case "already_accepted":
				return "Each link works once. Sign in with the account you created.";
			case "create_account":
				return `You were invited as ${roleLabels[invitation.role]}. Choose a name and a password to finish.`;
			case "sign_in_required":
				return `${invitation.email} already has an account here. Sign in, then open this link again.`;
			case "confirm":
				return `You were invited as ${roleLabels[invitation.role]}. There is nothing else to set up.`;
			case "address_mismatch":
				return `This invitation was issued to ${invitation.email}. Sign out and open the link again with that address.`;
			case "sso_required":
				return "Accept the invitation by signing in with your identity provider. There is no password to set.";
			case "joined":
				return `Welcome to ${invitation.workspace.name}.`;
			case "unavailable":
				return "We couldn't finish that just now. Wait a moment and try again.";
		}
	});

	const notice = $derived.by(() => {
		if (invitation.kind === "sso_required") {
			return {
				variant: "muted" as const,
				icon: Info,
				title: "Your identity provider owns this account",
				body: "Signing in through it accepts the invitation and creates your account if you have none.",
			};
		}
		if (invitation.kind === "joined") {
			return {
				variant: "success" as const,
				icon: CircleCheck,
				title: `You are a member of ${invitation.workspace.name}`,
				body: "The invitation link is spent now and cannot be used again.",
			};
		}
		if (invitation.kind === "unavailable") {
			return {
				variant: "destructive" as const,
				icon: CircleX,
				title: "The invitation could not be accepted",
				body: "Nothing changed. Open the link again in a moment.",
			};
		}
		return null;
	});

	const showForm = $derived(invitation.kind === "create_account");

	const passwordRules = $derived([
		{
			met: $formData.password.length >= minPasswordLength,
			label: `At least ${minPasswordLength} characters`,
		},
		{ met: false, label: "Checked against known breaches on submit" },
	]);

	const action = $derived.by(() => {
		switch (invitation.kind) {
			case "create_account":
				return {
					label: busy ? "Joining" : "Create account and join",
					href: null,
					form: formId,
					onclick: null,
				};
			case "confirm":
				return {
					label: busy ? "Joining" : `Join ${invitation.workspace.name}`,
					href: null,
					form: null,
					onclick: confirmJoin,
				};
			case "sso_required":
				return { label: "Continue with single sign-on", href: "/sso", form: null, onclick: null };
			case "sign_in_required":
			case "already_accepted":
				return { label: "Sign in", href: "/sign-in", form: null, onclick: null };
			case "joined":
				return {
					label: `Open ${invitation.workspace.name}`,
					href: `/${invitation.workspace.slug}`,
					form: null,
					onclick: null,
				};
			default:
				return { label: "Back to sign in", href: "/sign-in", form: null, onclick: null };
		}
	});

	const footer = $derived(
		invitation.kind === "create_account" || invitation.kind === "confirm"
			? "Norn doesn't charge per seat, so nobody is paying for your account."
			: null
	);
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-form">
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

			{#if invitation.kind === "create_account" || invitation.kind === "confirm" || invitation.kind === "sign_in_required" || invitation.kind === "sso_required"}
				<div class="flex flex-col gap-2.5 rounded-lg border border-line-default bg-paper-1 p-3">
					<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
						<span class="font-mono text-xs break-all text-ink-900">{invitation.email}</span>
						<span class="font-mono text-xs text-muted-foreground">{workspaceName}</span>
					</div>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Invitations are issued to one address. Accepting adds this address to the workspace.
					</p>
				</div>
			{/if}

			{#if showForm}
				<form id={formId} method="POST" use:enhance class="flex flex-col gap-4">
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
				</form>
			{/if}

			<div class="flex flex-col gap-2">
				{#if action.href}
					<Button href={action.href} class="w-full">{action.label}</Button>
				{:else if action.form}
					<Button type="submit" form={action.form} class="w-full" disabled={busy}>
						{action.label}
					</Button>
				{:else}
					<Button class="w-full" disabled={busy} onclick={action.onclick}>{action.label}</Button>
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
