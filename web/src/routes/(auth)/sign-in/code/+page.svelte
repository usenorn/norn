<script lang="ts">
	import { page } from "$app/state";
	import { superForm } from "sveltekit-superforms";
	import { zod4Client } from "sveltekit-superforms/adapters";
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import TriangleAlert from "@lucide/svelte/icons/triangle-alert";
	import { authPath } from "$lib/auth/return-to";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import InstanceLine from "$lib/components/norn/instance-line.svelte";
	import { signInCodeLength, signInCodeSchema } from "$lib/auth/sign-in-code-schema";
	import type { SignInCodeFailure } from "$lib/auth/types";
	import { signInCodePreviewStates } from "./preview";
	import type { PageProps } from "./$types";

	let { data }: PageProps = $props();

	const preview = $derived(
		import.meta.env.DEV
			? signInCodePreviewStates[page.url.searchParams.get("state") ?? ""]
			: undefined
	);

	// svelte-ignore state_referenced_locally
	const form = superForm(data.form, {
		id: "sign-in-code",
		validators: zod4Client(signInCodeSchema),
		resetForm: false,
	});

	const { form: formData, enhance, submitting, message } = form;

	const failure = $derived<SignInCodeFailure | null>($message ?? preview?.failure ?? null);
	const spent = $derived(failure?.kind === "spent" || failure?.kind === "rate_limited");

	const notice = $derived.by(() => {
		if (!failure) return null;

		if (failure.kind === "incorrect") {
			return {
				variant: "destructive" as const,
				icon: CircleAlert,
				title: "That code is not the one we sent",
				body:
					failure.attemptsLeft === 1
						? "One more wrong code and this sign-in ends. Check the newest message — an older code will not work."
						: `Check the newest message and try again. ${failure.attemptsLeft} attempts left.`,
			};
		}

		if (failure.kind === "spent") {
			return {
				variant: "destructive" as const,
				icon: TriangleAlert,
				title: "This sign-in has ended",
				body: "The code lapsed or was guessed at too many times. Start again with your password.",
			};
		}

		if (failure.kind === "rate_limited") {
			return {
				variant: "destructive" as const,
				icon: TriangleAlert,
				title: "Too many attempts from here",
				body: "Wait a couple of minutes before trying again.",
			};
		}

		return {
			variant: "destructive" as const,
			icon: TriangleAlert,
			title: "Something went wrong",
			body: "We could not check that code just now. Wait a moment and try again.",
		};
	});
</script>

<svelte:head><title>Enter your code · Norn</title></svelte:head>

<div class="my-auto flex w-full flex-col items-center gap-4">
	<div class="notch w-full max-w-form">
		<div class="flex flex-col gap-4.5 p-6.5 pb-5.5">
			<div class="flex flex-col gap-1.5">
				<h1 class="text-2xl font-medium tracking-title text-ink-900">Enter your code</h1>
				<p class="text-md leading-normal text-muted-foreground text-pretty">
					We sent a {signInCodeLength}-digit code to your email. It works once and lasts ten
					minutes.
				</p>
			</div>

			{#if notice}
				{@const NoticeIcon = notice.icon}
				<Alert.Root variant={notice.variant}>
					<NoticeIcon aria-hidden="true" />
					<Alert.Title>{notice.title}</Alert.Title>
					<Alert.Description>{notice.body}</Alert.Description>
				</Alert.Root>
			{/if}

			<form method="POST" use:enhance class="flex flex-col gap-4">
				<input type="hidden" name="challengeId" value={$formData.challengeId} />

				<Form.Field {form} name="code">
					<Form.Control>
						{#snippet children({ props })}
							<Form.Label>Code</Form.Label>
							<Input
								{...props}
								inputmode="numeric"
								autocomplete="one-time-code"
								autocapitalize="none"
								spellcheck="false"
								maxlength={signInCodeLength}
								placeholder="000000"
								class="text-center font-mono text-lg tracking-[0.4em]"
								disabled={spent || $submitting}
								bind:value={$formData.code}
							/>
						{/snippet}
					</Form.Control>
					<Form.FieldErrors />
				</Form.Field>

				<Form.Button class="w-full" disabled={spent || $submitting}>
					{$submitting ? "Checking" : "Sign in"}
				</Form.Button>
			</form>
		</div>
	</div>

	<div class="flex w-full max-w-form flex-col items-center gap-2">
		<p class="text-center text-sm leading-normal text-muted-foreground text-pretty">
			No message yet? It can take a moment. Check spam before asking an admin.
		</p>
		<a
			href={authPath(page.url, "/sign-in")}
			class="text-sm text-link hover:text-link-hover hover:underline"
		>
			Start again with your password
		</a>
		<InstanceLine instance={data.auth} />
	</div>
</div>
