<script lang="ts">
	import CircleAlert from "@lucide/svelte/icons/circle-alert";
	import { defaults, setError, superForm } from "sveltekit-superforms";
	import { zod4, zod4Client } from "sveltekit-superforms/adapters";
	import { api } from "$lib/api";
	import * as Alert from "$lib/components/ui/alert/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import * as Form from "$lib/components/ui/form/index.js";
	import * as RadioGroup from "$lib/components/ui/radio-group/index.js";
	import * as Tabs from "$lib/components/ui/tabs/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Textarea } from "$lib/components/ui/textarea/index.js";
	import {
		capabilityFailure,
		capabilityFailureMessage,
		fieldProblems,
		skillFieldMessage,
		type AgentCapabilityFailure,
		type AgentSkill,
		type SkillDialogMode,
		type SkillSourceDiscovery,
	} from "./agent-capabilities";
	import { archiveBase64, skillSourceSchema, skillWriteSchema } from "./agent-capability-schemas";

	let {
		open = $bindable(false),
		workspaceId,
		agentId,
		mode,
		onsaved,
	}: {
		open?: boolean;
		workspaceId: string;
		agentId?: string;
		mode: SkillDialogMode;
		onsaved: (skill: AgentSkill) => void;
	} = $props();

	type Discovery =
		| { kind: "idle" }
		| { kind: "resolving" }
		| { kind: "resolved"; source: string; discovery: SkillSourceDiscovery };

	let tab = $state<"import" | "write">("import");
	let discovery = $state.raw<Discovery>({ kind: "idle" });
	let failure = $state<AgentCapabilityFailure | null>(null);

	const importForm = superForm(defaults(zod4(skillSourceSchema)), {
		id: "skill-import-form",
		SPA: true,
		validators: zod4Client(skillSourceSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			failure = null;

			if (discovery.kind !== "resolved" || discovery.source !== entered.data.source) {
				await resolve(entered);

				return;
			}

			await added(entered, "source", () =>
				post({ kind: "import", source: entered.data.source, path: entered.data.path })
			);
		},
	});

	const writeForm = superForm(defaults(zod4(skillWriteSchema)), {
		id: "skill-write-form",
		SPA: true,
		validators: zod4Client(skillWriteSchema),
		resetForm: false,
		onUpdate: async ({ form: entered }) => {
			if (!entered.valid) return;

			failure = null;

			const archive = entered.data.archive ? await archiveBase64(entered.data.archive) : undefined;
			const instructions = archive ? undefined : entered.data.instructions;

			await added(entered, "instructions", () =>
				mode.kind === "rewrite"
					? api.PATCH("/workspaces/{workspaceId}/agent-skills/{skillId}", {
							params: { path: { workspaceId, skillId: mode.skill.id } },
							body: { instructions, archive },
						})
					: post({ kind: "manual", instructions, archive })
			);
		},
	});

	const { form: importData, enhance: importEnhance, submitting: importing } = importForm;
	const { form: writeData, enhance: writeEnhance, submitting: writing } = writeForm;

	const busy = $derived($importing || $writing || discovery.kind === "resolving");
	const candidates = $derived.by(() => {
		if (discovery.kind !== "resolved") return [];

		const suggested = discovery.discovery.suggested;

		return [...discovery.discovery.candidates].sort(
			(a, b) => Number(b.path === suggested) - Number(a.path === suggested)
		);
	});
	const stale = $derived(discovery.kind === "resolved" && discovery.source !== $importData.source.trim());

	$effect(() => {
		if (!open) return;

		failure = null;
		discovery = { kind: "idle" };
		tab = mode.kind === "rewrite" ? "write" : "import";
		importForm.reset({ keepMessage: false });
		writeForm.reset({ keepMessage: false });

		if (mode.kind === "rewrite") $writeData.instructions = mode.skill.instructions;
	});

	function post(body: {
		kind: "import" | "manual";
		source?: string;
		path?: string;
		instructions?: string;
		archive?: string;
	}) {
		return agentId
			? api.POST("/workspaces/{workspaceId}/agents/{agentId}/skills", {
					params: { path: { workspaceId, agentId } },
					body,
				})
			: api.POST("/workspaces/{workspaceId}/agent-library/skills", {
					params: { path: { workspaceId } },
					body,
				});
	}

	async function resolve(entered: Parameters<typeof setError>[0] & { data: { source: string; path: string } }) {
		const source = entered.data.source.trim();
		discovery = { kind: "resolving" };

		try {
			const { data, error, response } = await api.GET("/workspaces/{workspaceId}/skill-sources", {
				params: { path: { workspaceId }, query: { source } },
			});

			if (error || !data) {
				discovery = { kind: "idle" };
				report(entered, "source", error, response.status);

				return;
			}

			discovery = { kind: "resolved", source, discovery: data };
			entered.data.path = data.suggested ?? (data.candidates.length === 1 ? data.candidates[0].path : "");
		} catch {
			discovery = { kind: "idle" };
			failure = { kind: "unavailable" };
		}
	}

	async function added(
		entered: Parameters<typeof setError>[0],
		field: "source" | "instructions",
		call: () => Promise<{ data?: AgentSkill; error?: unknown; response: Response }>
	) {
		try {
			const { data, error, response } = await call();

			if (error || !data) {
				report(entered, field, error, response.status);

				return;
			}

			onsaved(data);
			open = false;
		} catch {
			failure = { kind: "unavailable" };
		}
	}

	function report(
		entered: Parameters<typeof setError>[0],
		field: "source" | "instructions",
		error: unknown,
		status: number
	) {
		const problems = fieldProblems(error);

		if (problems.length > 0) {
			for (const problem of problems) {
				setError(entered, problem.field === "path" ? "path" : field, skillFieldMessage(problem));
			}

			return;
		}

		failure = capabilityFailure(error, status);
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content variant="scrollable" class="sm:max-w-140">
		<Dialog.Header>
			<Dialog.Title>{mode.kind === "rewrite" ? `Edit ${mode.skill.name}` : "Add a skill"}</Dialog.Title>
			<Dialog.Description>
				{mode.kind === "rewrite"
					? "Replace the skill's SKILL.md, or upload the whole folder again."
					: agentId
						? "The agent loads a skill when a task calls for it. Import one from GitHub or skills.sh, or write your own."
						: "Library skills can be given to any agent in this workspace."}
			</Dialog.Description>
		</Dialog.Header>

		{#if failure}
			<Alert.Root variant="destructive">
				<CircleAlert aria-hidden="true" />
				<Alert.Title>The skill was not saved</Alert.Title>
				<Alert.Description>{capabilityFailureMessage(failure)}</Alert.Description>
			</Alert.Root>
		{/if}

		<Tabs.Root bind:value={tab} class="gap-4">
			{#if mode.kind === "add"}
				<Tabs.List variant="line" class="w-full justify-start">
					<Tabs.Trigger value="import" class="flex-none" disabled={busy}>GitHub or skills.sh</Tabs.Trigger>
					<Tabs.Trigger value="write" class="flex-none" disabled={busy}>Write or upload</Tabs.Trigger>
				</Tabs.List>
			{/if}

			<Tabs.Content value="import">
				<form method="POST" id="skill-import-form" use:importEnhance class="flex flex-col gap-5">
					<Form.Field form={importForm} name="source">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Source</Form.Label>
								<Input
									{...props}
									bind:value={$importData.source}
									disabled={busy}
									placeholder="anthropics/skills or https://skills.sh/anthropics/skills/pdf"
									autocomplete="off"
									spellcheck="false"
								/>
								<Form.Description>
									A GitHub repository, a github.com or skills.sh address, or an
									<code class="font-mono text-2xs">npx skills add</code> command. Public repositories only.
								</Form.Description>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					{#if discovery.kind === "resolved" && !stale}
						<Form.Fieldset form={importForm} name="path">
							<Form.Legend>
								{candidates.length === 1 ? "Skill found" : `${candidates.length} skills found`}
								<span class="ml-1 font-mono text-2xs text-muted-foreground">
									{discovery.discovery.repository} @ {discovery.discovery.revision.slice(0, 7)}
								</span>
							</Form.Legend>
							<RadioGroup.Root name="path" bind:value={$importData.path} disabled={busy} class="gap-0">
								<ul class="max-h-72 divide-y divide-line-subtle overflow-auto border border-line-subtle bg-paper-1">
									{#each candidates as candidate (candidate.path)}
										<li class="flex items-start gap-3 p-3">
											<RadioGroup.Item
												id={`skill-candidate-${candidate.path || "root"}`}
												value={candidate.path}
												class="mt-0.5"
											/>
											<label for={`skill-candidate-${candidate.path || "root"}`} class="min-w-0 flex-1">
												<span class="block text-sm text-ink-900">{candidate.name}</span>
												<span class="mt-0.5 line-clamp-2 text-xs text-muted-foreground">
													{candidate.description}
												</span>
												<span class="mt-0.5 block font-mono text-2xs text-muted-foreground">
													{candidate.path || "repository root"}
												</span>
											</label>
										</li>
									{/each}
								</ul>
							</RadioGroup.Root>
							<Form.FieldErrors />
						</Form.Fieldset>
					{/if}

					<Dialog.Footer>
						<Button type="button" variant="secondary" disabled={busy} onclick={() => (open = false)}>Cancel</Button>
						<Form.Button disabled={busy}>
							{discovery.kind === "resolving"
								? "Looking…"
								: $importing
									? "Adding…"
									: discovery.kind === "resolved" && !stale
										? "Add skill"
										: "Find skills"}
						</Form.Button>
					</Dialog.Footer>
				</form>
			</Tabs.Content>

			<Tabs.Content value="write">
				<form
					method="POST"
					id="skill-write-form"
					enctype="multipart/form-data"
					use:writeEnhance
					class="flex flex-col gap-5"
				>
					<Form.Field form={writeForm} name="instructions">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>SKILL.md</Form.Label>
								<Textarea
									{...props}
									bind:value={$writeData.instructions}
									disabled={busy || $writeData.archive !== undefined}
									rows={12}
									class="font-mono text-xs"
									spellcheck="false"
									placeholder={"---\nname: release-notes\ndescription: Writes release notes from merged pull requests.\n---\n\nRead the merged pull requests since the last tag…"}
								/>
								<Form.Description>
									Frontmatter with a lowercase name and a description, then the instructions.
								</Form.Description>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Form.Field form={writeForm} name="archive">
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>Or upload the skill folder</Form.Label>
								<Input
									{...props}
									type="file"
									accept=".zip,.skill,application/zip"
									disabled={busy}
									onchange={(event) => {
										$writeData.archive = event.currentTarget.files?.[0] ?? undefined;
									}}
								/>
								<Form.Description>
									A zip with SKILL.md at its root or in one top folder, with any scripts and references it
									uses. Up to 3 MB.
								</Form.Description>
							{/snippet}
						</Form.Control>
						<Form.FieldErrors />
					</Form.Field>

					<Dialog.Footer>
						<Button type="button" variant="secondary" disabled={busy} onclick={() => (open = false)}>Cancel</Button>
						<Form.Button disabled={busy}>
							{$writing ? "Saving…" : mode.kind === "rewrite" ? "Save skill" : "Add skill"}
						</Form.Button>
					</Dialog.Footer>
				</form>
			</Tabs.Content>
		</Tabs.Root>
	</Dialog.Content>
</Dialog.Root>
