<script lang="ts">
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import InstanceLine from "$lib/components/norn/instance-line.svelte";
	import { api } from "$lib/api";
	import { workspaceEntrySchema } from "$lib/auth/workspace-entry-schema";
	import { reachWorkspaceSignIn, ssoEntryPoint, workspaceSlug } from "$lib/auth/workspace-sign-in";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const returnTo = $derived.by(() => {
		const requested = page.url.searchParams.get("return") ?? "";

		return requested.startsWith("/") && !requested.startsWith("//") ? requested : "/";
	});

	const form = superForm(defaults(zod4(workspaceEntrySchema)), {
		id: "workspace-entry",
		SPA: true,
		validators: zod4Client(workspaceEntrySchema),
		resetForm: false,
		onUpdate: async ({ form: asked }) => {
			const slug = workspaceSlug(asked.data.workspace);

			if (!asked.valid) return;

			if (!slug) {
				setError(asked, "workspace", "Enter the workspace address you sign in at.");

				return;
			}

			const entry = await reachWorkspaceSignIn(api, slug);

			if (entry.kind !== "ready") {
				setError(
					asked,
					"workspace",
					"No workspace at that address. Check it with whoever invited you."
				);

				return;
			}

			await goto(
				entry.signIn.sso
					? ssoEntryPoint(entry.signIn.workspace, returnTo)
					: `/sign-in?workspace=${entry.signIn.workspace}`
			);
		},
	});

	const { form: formData, enhance, submitting } = form;
</script>

<svelte:head><title>Single sign-on · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-form">
		<div class="flex flex-col gap-4.5 p-6.5 pb-5.5">
			<div class="flex flex-col gap-1.5">
				<h1 class="text-2xl font-medium tracking-title text-ink-900">Single sign-on</h1>
				<p class="text-md leading-normal text-muted-foreground text-pretty">
					Enter the address your team signs in at. We will send you to whichever provider that
					workspace uses.
				</p>
			</div>

			<form method="POST" use:enhance class="flex flex-col gap-4">
				<Form.Field {form} name="workspace">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Workspace address</Form.Label>
							<Input
								{...props}
								autocapitalize="none"
								autocomplete="organization"
								spellcheck="false"
								placeholder="northwind"
								disabled={$submitting}
								bind:value={$formData.workspace}
							/>
						{/snippet}
					</Form.Control>
					<Form.Description>
						The last part of the link your team signs in at, like {data.auth.host}/northwind.
					</Form.Description>
					<Form.FieldErrors />
				</Form.Field>

				<Form.Button class="w-full" disabled={$submitting}>
					{$submitting ? "Looking" : "Continue"}
				</Form.Button>
			</form>
		</div>
	</div>

	<div class="flex w-full max-w-form flex-col items-center gap-2">
		<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">
			Not sure which address? Whoever invited you can send the link again.
		</p>
		<a href="/sign-in" class="text-sm text-link hover:text-link-hover hover:underline">
			Sign in with a password instead
		</a>
		<InstanceLine instance={data.auth} />
	</div>
</div>
