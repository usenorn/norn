<script lang="ts">
	import { page } from "$app/state";
	import { enhance as confirmEnhance } from "$app/forms";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleCheck from "@lucide/svelte/icons/circle-check";
	import CircleDashed from "@lucide/svelte/icons/circle-dashed";
	import CircleX from "@lucide/svelte/icons/circle-x";
	import Info from "@lucide/svelte/icons/info";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Avatar from "$lib/components/ui/avatar/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import WorkspaceMark from "$lib/components/norn/workspace-mark.svelte";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import InstanceLine from "$lib/components/norn/instance-line.svelte";
	import { minPasswordLength } from "$lib/auth/sign-up-schema";
	import { acceptInvitationSchema } from "$lib/workspace/accept-invitation-schema";
	import {
		invitedHeadline,
		linkFailure,
		type AcceptInvitation,
		type InvitationContext,
		type InvitationDetail,
	} from "$lib/workspace/accept-invitation";
	import { initialsOf } from "$lib/team/members";
	import { onDate } from "$lib/time";
	import { roleLabels } from "$lib/workspace/members";
	import { acceptInvitationPreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	const formId = "accept-invitation-form";
	const confirmFormId = "confirm-invitation-form";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? acceptInvitationPreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	let timezone = $state("UTC");

	const auth = $derived(data.auth);

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		id: formId,
		validators: zod4Client(acceptInvitationSchema),
		resetForm: false,
	});
	const { form: formData, enhance, submitting, message } = form;

	$effect(() => {
		timezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
	});

	const invitation = $derived<AcceptInvitation>($message ?? preview ?? data.invitation);

	const detail = $derived<InvitationDetail | undefined>(
		invitation.kind === "create_account" ||
			invitation.kind === "confirm" ||
			invitation.kind === "sign_in_required" ||
			invitation.kind === "sso_required"
			? invitation
			: undefined
	);

	const context = $derived<InvitationContext | undefined>(detail);

	const inviterName = $derived(detail?.invitedBy?.name ?? detail?.workspace.name ?? "");

	const invitedMeta = $derived(
		detail
			? [detail.invitedBy?.email, `sent ${onDate(detail.invitedAt, "UTC")}`]
					.filter(Boolean)
					.join(" · ")
			: ""
	);

	const workspaceMeta = $derived.by(() => {
		if (!detail) return "";

		const teams = (detail.teams ?? []).map((team) => `${team} team`);
		const role = "role" in invitation ? roleLabels[invitation.role] : "";

		return [...teams, role].filter(Boolean).join(" · ");
	});

	const note = $derived.by(() => {
		if (invitation.kind === "create_account") {
			const team = invitation.teams[0];

			return team
				? `You'll land in the ${team} team. That can change any time.`
				: "You can be put on a team once you are in.";
		}

		if (invitation.kind === "confirm") {
			return "You keep your account and switch workspaces from the sidebar.";
		}

		return null;
	});

	const busy = $derived($submitting);

	const passwordHint = $derived(
		$formData.password.length > 0 && $formData.password.length < minPasswordLength
			? `${minPasswordLength} characters minimum. This one is shorter.`
			: `${minPasswordLength} characters minimum.`
	);



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
					form: confirmFormId,
					onclick: null,
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

	const footer = $derived.by(() => {
		const until = detail ? onDate(detail.expiresAt, "UTC") : "";

		if (invitation.kind === "create_account") {
			return `The invitation is good until ${until}. Accounts exist only by invitation.`;
		}

		if (invitation.kind === "confirm") return `The invitation is good until ${until}.`;

		if (invitation.kind === "already_accepted") {
			return "Forgotten the password? Reset it from the sign-in screen.";
		}

		return null;
	});
</script>

<svelte:head><title>{title} · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-100">
		<div class="flex flex-col gap-4.5 p-6 pb-5.5">
			{#if detail}
				<div class="flex items-center gap-2.5 border-b border-line-subtle pb-4">
					<Avatar.Root size="default" class="flex-none">
						<Avatar.Fallback>{initialsOf(inviterName)}</Avatar.Fallback>
					</Avatar.Root>
					<div class="flex min-w-0 flex-col gap-0.5">
						<span class="text-md text-ink-900">{invitedHeadline(detail)}</span>
						<span class="font-mono text-xs break-all text-muted-foreground">{invitedMeta}</span>
					</div>
				</div>

				<div class="flex items-center gap-2.5">
					<WorkspaceMark name={detail.workspace.name} class="size-7 flex-none text-base" />
					<div class="flex min-w-0 flex-col gap-px">
						<span class="text-base font-medium tracking-snug text-ink-900">
							{detail.workspace.name}
						</span>
						<span class="font-mono text-xs text-muted-foreground">{workspaceMeta}</span>
					</div>
				</div>
			{:else}
				<div class="flex flex-col gap-1.5">
					<h1 class="text-2xl font-medium tracking-title text-ink-900">{title}</h1>
					<p class="text-md leading-normal text-muted-foreground text-pretty">{lede}</p>
				</div>
			{/if}

			{#if notice}
				{@const NoticeIcon = notice.icon}
				<Alert.Root variant={notice.variant}>
					<NoticeIcon aria-hidden="true" />
					<Alert.Title>{notice.title}</Alert.Title>
					<Alert.Description>{notice.body}</Alert.Description>
				</Alert.Root>
			{/if}

			{#if showForm}
				<form id={formId} method="POST" action="?/join" use:enhance class="flex flex-col gap-4">
					<input type="hidden" name="timezone" value={timezone} />
					<input type="hidden" name="token" value={data.token ?? ""} />
					{#if $formData.terms}
						<input type="hidden" name="terms" value="true" />
					{/if}
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

					<div class="flex flex-col gap-1">
						<span class="font-mono text-2xs font-medium tracking-caps text-ink-600 uppercase">
							Email
						</span>
						<Input value={invitation.kind === "create_account" ? invitation.email : ""} disabled />
						<span class="text-sm text-muted-foreground">Fixed by the invitation.</span>
					</div>

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
						<Form.Description>{passwordHint}</Form.Description>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field {form} name="repeat">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Repeat password</Form.Label>
								<Input
									{...props}
									type="password"
									autocomplete="new-password"
									disabled={busy}
									bind:value={$formData.repeat}
								/>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>
				</form>
			{/if}

			{#if invitation.kind === "confirm"}
				<div
					class="flex items-center gap-2.5 rounded-lg border border-line-default bg-paper-1 px-3 py-2.5"
				>
					<Avatar.Root size="sm" class="flex-none">
						<Avatar.Fallback>{initialsOf(invitation.email)}</Avatar.Fallback>
					</Avatar.Root>
					<div class="flex min-w-0 flex-1 flex-col gap-px">
						<span class="truncate text-md text-ink-900">{data.signedInAs || invitation.email}</span>
						<span class="font-mono text-xs break-all text-muted-foreground">{invitation.email}</span>
					</div>
					<a
						href="/sign-in?return={encodeURIComponent(page.url.pathname + page.url.search)}"
						class="flex-none text-sm text-link hover:text-link-hover hover:underline"
					>
						Switch
					</a>
				</div>
			{/if}

			<div class="flex flex-col gap-3">
				{#if showForm}
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
				{/if}

				{#if invitation.kind === "confirm"}
					<form id={confirmFormId} method="POST" action="?/confirm" use:confirmEnhance>
						<input type="hidden" name="timezone" value={timezone} />
						<input type="hidden" name="token" value={data.token ?? ""} />
					</form>
				{/if}

				{#if action.href}
					<Button href={action.href} class="w-full">{action.label}</Button>
				{:else if action.form}
					<Button type="submit" form={action.form} class="w-full" disabled={busy}>
						{action.label}
					</Button>
				{/if}

				{#if note}
					<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">{note}</p>
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
