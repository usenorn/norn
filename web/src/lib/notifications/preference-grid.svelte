<script lang="ts">
	import Bot from "@lucide/svelte/icons/bot";
	import { Checkbox } from "$lib/components/ui/checkbox/index.js";
	import { Label } from "$lib/components/ui/label/index.js";
	import { preferenceRows, type NotificationPreferences } from "./notifications";

	let {
		preferences,
		emailEnabled,
		disabled = false,
		idPrefix,
		onchange,
	}: {
		preferences: NotificationPreferences;
		emailEnabled: boolean;
		disabled?: boolean;
		idPrefix: string;
		onchange: (next: NotificationPreferences) => void;
	} = $props();

	function toggle(key: keyof NotificationPreferences, channel: "inbox" | "email", on: boolean) {
		onchange({
			...preferences,
			[key]: { ...preferences[key], [channel]: on },
		});
	}
</script>

<div class="overflow-x-auto">
	<table class="w-full min-w-90 border-collapse">
		<thead>
			<tr class="border-b border-line-default">
				<th class="py-2 pr-3 text-left font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase">
					Event
				</th>
				<th class="w-16 py-2 text-center font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase">
					Inbox
				</th>
				<th class="w-16 py-2 text-center font-mono text-2xs font-medium tracking-eyebrow text-ink-600 uppercase">
					Email
				</th>
			</tr>
		</thead>
		<tbody>
			{#each preferenceRows as row (row.key)}
				<tr class="border-b border-line-subtle">
					<td class="py-2.5 pr-3">
						<Label for="{idPrefix}-{row.key}-inbox" class="text-md font-medium tracking-snug text-ink-900">
							{row.label}
						</Label>
						<p class="text-sm leading-normal text-muted-foreground text-pretty">{row.description}</p>
					</td>
					<td class="py-2.5 text-center">
						<Checkbox
							id="{idPrefix}-{row.key}-inbox"
							{disabled}
							checked={preferences[row.key].inbox}
							onCheckedChange={(on) => toggle(row.key, "inbox", on === true)}
							aria-label="{row.label} in your inbox"
						/>
					</td>
					<td class="py-2.5 text-center">
						<Checkbox
							id="{idPrefix}-{row.key}-email"
							disabled={disabled || !emailEnabled}
							checked={preferences[row.key].email}
							onCheckedChange={(on) => toggle(row.key, "email", on === true)}
							aria-label="{row.label} by email"
						/>
					</td>
				</tr>
			{/each}
			<tr>
				<td class="py-2.5 pr-3">
					<Label
						for="{idPrefix}-agents-inbox"
						class="flex items-center gap-1.5 text-md font-medium tracking-snug text-ink-900"
					>
						<Bot class="size-icon-row shrink-0" aria-hidden="true" />
						Agent activity
					</Label>
					<p class="text-sm leading-normal text-muted-foreground text-pretty">
						Turn this off to stop hearing about changes an agent made, without silencing anyone else.
					</p>
				</td>
				<td class="py-2.5 text-center">
					<Checkbox
						id="{idPrefix}-agents-inbox"
						{disabled}
						checked={preferences.agents.inbox}
						onCheckedChange={(on) => toggle("agents", "inbox", on === true)}
						aria-label="Agent activity in your inbox"
					/>
				</td>
				<td class="py-2.5 text-center">
					<Checkbox
						id="{idPrefix}-agents-email"
						disabled={disabled || !emailEnabled}
						checked={preferences.agents.email}
						onCheckedChange={(on) => toggle("agents", "email", on === true)}
						aria-label="Agent activity by email"
					/>
				</td>
			</tr>
		</tbody>
	</table>
</div>
