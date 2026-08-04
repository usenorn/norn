<script lang="ts">
	import { invalidateAll } from "$app/navigation";
	import { defaults, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import Copy from "@lucide/svelte/icons/copy";
	import ShieldAlert from "@lucide/svelte/icons/shield-alert";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import Eyebrow from "$lib/components/norn/eyebrow.svelte";
	import { api } from "$lib/api";
	import { onDate } from "$lib/time";
	import { samlConnectionSchema } from "$lib/workspace/saml-schema";
	import {
		certificateAdvice,
		certificateLine,
		certificateUrgent,
		saveFailure,
		type SamlConnection,
		type SsoFailure,
	} from "$lib/workspace/sso";

	const formId = "saml-connection-form";

	let {
		workspace,
		connection,
		busy = false,
		onfailure,
		onsaved,
		onbusy,
	}: {
		workspace: { id: string; name: string; slug: string; timezone: string };
		connection: SamlConnection | null;
		busy?: boolean;
		onfailure: (failure: SsoFailure | null) => void;
		onsaved: () => void;
		onbusy: (working: boolean) => void;
	} = $props();

	let testing = $state(false);
	let copied = $state(false);

	const form = superForm(defaults(zod4(samlConnectionSchema)), {
		id: formId,
		SPA: true,
		validators: zod4Client(samlConnectionSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			onfailure(null);

			try {
				const { data: result, error } = await api.PUT("/workspaces/{workspaceId}/sso/saml", {
					params: { path: { workspaceId: workspace.id } },
					body: {
						metadataUrl: entered.data.source === "url" ? entered.data.metadataUrl : undefined,
						metadata: entered.data.source === "paste" ? entered.data.metadata : undefined,
						descriptor:
							entered.data.source === "manual"
								? {
										entityId: entered.data.entityId,
										ssoUrl: entered.data.ssoUrl,
										certificates: [entered.data.certificate],
										expiresAt: new Date().toISOString(),
									}
								: undefined,
						allowIdpInitiated: entered.data.allowIdpInitiated,
						provisioning: entered.data.provisioning,
						mapping: {
							email: entered.data.emailAttribute || undefined,
							name: entered.data.nameAttribute || undefined,
							groups: entered.data.groupsAttribute || undefined,
						},
					},
				});

				if (error) {
					onfailure(saveFailure(error));

					return;
				}

				if (result) {
					onsaved();
					await invalidateAll();
				}
			} catch {
				onfailure({ kind: "unavailable" });
			}
		},
	});
	const { form: formData, enhance, submitting } = form;

	const working = $derived(busy || testing || $submitting);

	$effect(() => onbusy($submitting || testing));

	$effect(() => {
		if (!connection) return;

		formData.update(
			(current) => ({
				...current,
				source: connection.providerMetadataUrl ? "url" : "manual",
				metadataUrl: connection.providerMetadataUrl ?? "",
				entityId: connection.descriptor.entityId,
				ssoUrl: connection.descriptor.ssoUrl,
				certificate: connection.descriptor.certificates[0] ?? "",
				emailAttribute: connection.mapping.email ?? "",
				nameAttribute: connection.mapping.name ?? "",
				groupsAttribute: connection.mapping.groups ?? "",
				allowIdpInitiated: connection.allowIdpInitiated,
				provisioning: connection.provisioning,
			}),
			{ taint: false }
		);
	});

	async function test() {
		testing = true;
		onfailure(null);

		try {
			const { data, error } = await api.POST("/workspaces/{workspaceId}/sso/saml/test", {
				params: { path: { workspaceId: workspace.id } },
			});

			if (error) {
				onfailure(saveFailure(error));

				return;
			}

			if (data) window.location.assign(data.authorizationUrl);
		} catch {
			onfailure({ kind: "unavailable" });
		} finally {
			testing = false;
		}
	}

	async function copyMetadata() {
		if (!connection) return;

		await navigator.clipboard.writeText(connection.metadataUrl);
		copied = true;
	}
</script>

{#if connection}
	{@const daysLeft = connection.certificateDaysLeft}
	{#if certificateUrgent(daysLeft)}
		<Alert.Root variant={daysLeft !== undefined && daysLeft < 0 ? "destructive" : "warning"}>
			<ShieldAlert aria-hidden="true" />
			<Alert.Title>
				Your provider's certificate — {certificateLine(daysLeft)}
			</Alert.Title>
			<Alert.Description>
				{certificateAdvice(daysLeft)} Copy the current certificate from {connection.descriptor
					.entityId} and save it here.
			</Alert.Description>
		</Alert.Root>
	{/if}

	<div class="flex flex-col gap-1 rounded-lg border border-line-subtle p-3">
		<div class="flex items-baseline justify-between gap-2">
			<Eyebrow class="text-ink-600">Norn's metadata</Eyebrow>
			<Button variant="ghost" size="sm" onclick={copyMetadata}>
				<Copy aria-hidden="true" />
				{copied ? "Copied" : "Copy"}
			</Button>
		</div>
		<span class="font-mono text-sm break-all text-ink-900">{connection.metadataUrl}</span>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Give this address to your provider to register Norn. It carries the callback address and
			Norn's signing certificate.
		</p>
	</div>

	<div class="flex flex-col gap-1 rounded-lg border border-line-subtle p-3">
		<Eyebrow class="text-ink-600">Certificate</Eyebrow>
		<span class="text-md text-ink-900">
			{onDate(connection.certificateExpiresAt, workspace.timezone)} — {certificateLine(daysLeft)}
		</span>
		<p class="text-sm leading-normal text-muted-foreground text-pretty">
			Norn warns every administrator by email as this date approaches, so nobody discovers it
			through a failed sign-in.
		</p>
	</div>
{/if}

<form id={formId} method="POST" use:enhance class="flex flex-col gap-4">
	<Form.Field {form} name="source">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Where the provider's details come from</Form.Label>
				<div {...props} class="flex flex-wrap gap-2">
					{#each [["url", "Metadata address"], ["paste", "Paste metadata"], ["manual", "Enter by hand"]] as [value, label] (value)}
						<Button
							type="button"
							variant={$formData.source === value ? "default" : "secondary"}
							size="sm"
							disabled={working}
							onclick={() => ($formData.source = value as typeof $formData.source)}
						>
							{label}
						</Button>
					{/each}
				</div>
			{/snippet}
		</Form.Control>
	</Form.Field>

	{#if $formData.source === "url"}
		<Form.Field {form} name="metadataUrl">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Metadata address</Form.Label>
					<Input
						{...props}
						placeholder="https://login.example.com/app/saml/metadata"
						autocapitalize="none"
						spellcheck="false"
						disabled={working}
						bind:value={$formData.metadataUrl}
					/>
				{/snippet}
			</Form.Control>
			<Form.Description class="text-sm text-muted-foreground">
				Norn fetches this and reads the entity ID, sign-in URL and signing certificate from it.
			</Form.Description>
			<Form.FieldErrors />
		</Form.Field>
	{:else if $formData.source === "paste"}
		<Form.Field {form} name="metadata">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Provider metadata</Form.Label>
					<Textarea
						{...props}
						rows={8}
						placeholder="<EntityDescriptor …>"
						spellcheck="false"
						disabled={working}
						bind:value={$formData.metadata}
					/>
				{/snippet}
			</Form.Control>
			<Form.Description class="text-sm text-muted-foreground">
				For a provider that will not let this instance reach its metadata address.
			</Form.Description>
			<Form.FieldErrors />
		</Form.Field>
	{:else}
		<Form.Field {form} name="entityId">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Entity ID</Form.Label>
					<Input
						{...props}
						autocapitalize="none"
						spellcheck="false"
						disabled={working}
						bind:value={$formData.entityId}
					/>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>

		<Form.Field {form} name="ssoUrl">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Sign-in URL</Form.Label>
					<Input
						{...props}
						autocapitalize="none"
						spellcheck="false"
						disabled={working}
						bind:value={$formData.ssoUrl}
					/>
				{/snippet}
			</Form.Control>
			<Form.FieldErrors />
		</Form.Field>

		<Form.Field {form} name="certificate">
			<Form.Control>
				{#snippet children({ props })}
					<Form.Label>Signing certificate</Form.Label>
					<Textarea
						{...props}
						rows={6}
						spellcheck="false"
						disabled={working}
						bind:value={$formData.certificate}
					/>
				{/snippet}
			</Form.Control>
			<Form.Description class="text-sm text-muted-foreground">
				PEM or the bare base64 from the provider's metadata. Norn reads the expiry from it.
			</Form.Description>
			<Form.FieldErrors />
		</Form.Field>
	{/if}

	<Form.Field {form} name="emailAttribute">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Email attribute</Form.Label>
				<Input
					{...props}
					placeholder="Recognised automatically"
					autocapitalize="none"
					spellcheck="false"
					disabled={working}
					bind:value={$formData.emailAttribute}
				/>
			{/snippet}
		</Form.Control>
		<Form.Description class="text-sm text-muted-foreground">
			Leave blank unless your provider uses an unusual name. Norn already recognises email, mail,
			the LDAP OID and the AD FS claim.
		</Form.Description>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name="nameAttribute">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Name attribute</Form.Label>
				<Input
					{...props}
					placeholder="Recognised automatically"
					autocapitalize="none"
					spellcheck="false"
					disabled={working}
					bind:value={$formData.nameAttribute}
				/>
			{/snippet}
		</Form.Control>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name="groupsAttribute">
		<Form.Control>
			{#snippet children({ props })}
				<Form.Label>Groups attribute</Form.Label>
				<Input
					{...props}
					placeholder="Recognised automatically"
					autocapitalize="none"
					spellcheck="false"
					disabled={working}
					bind:value={$formData.groupsAttribute}
				/>
			{/snippet}
		</Form.Control>
		<Form.Description class="text-sm text-muted-foreground">
			Groups are read and stored; mapping them to teams is not built yet.
		</Form.Description>
		<Form.FieldErrors />
	</Form.Field>

	<Form.Field {form} name="allowIdpInitiated">
		<Form.Control>
			{#snippet children({ props })}
				<div class="flex items-start gap-2">
					<Checkbox {...props} disabled={working} bind:checked={$formData.allowIdpInitiated} />
					<div class="flex flex-col gap-0.5">
						<Form.Label>Accept sign-ins started by the provider</Form.Label>
						<span class="text-sm leading-normal text-muted-foreground text-pretty">
							Let people start from your provider's app launcher. Norn cannot tie these to a
							request it made, so leave this off unless you need it.
						</span>
					</div>
				</div>
			{/snippet}
		</Form.Control>
	</Form.Field>

	<Form.Field {form} name="provisioning">
		<Form.Control>
			{#snippet children({ props })}
				<div class="flex items-start gap-2">
					<Checkbox {...props} disabled={working} bind:checked={$formData.provisioning} />
					<div class="flex flex-col gap-0.5">
						<Form.Label>Create accounts on first sign-in</Form.Label>
						<span class="text-sm leading-normal text-muted-foreground text-pretty">
							Anyone your provider vouches for gets a Norn account and joins {workspace.name} as a
							member.
						</span>
					</div>
				</div>
			{/snippet}
		</Form.Control>
	</Form.Field>
</form>

<div class="flex flex-wrap gap-2">
	<Button type="submit" form={formId} disabled={working}>
		{$submitting ? "Saving" : connection ? "Save changes" : "Save provider"}
	</Button>

	{#if connection}
		<Button type="button" variant="secondary" disabled={working} onclick={test}>
			{testing ? "Opening your provider" : "Test connection"}
		</Button>
	{/if}
</div>
