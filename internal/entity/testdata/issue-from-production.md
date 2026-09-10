On a phone the bottom bar has four tabs. The fourth, **Menu**, points at `/{workspace}/menu`, and no such route exists — following it gives a 404.

`web/src/lib/workspace/navigation.ts:72`:

```ts
export function mobileNav(workspace: string): NavEntry[] {
	return [
		{ label: "Inbox", href: at("/inbox"), icon: Inbox },
		{ label: "Menu", href: at("/menu"), icon: Menu },
	];
}
```

There is no `menu` directory under `web/src/routes/[workspace]/(dashboard)`. Requested against the running app, `/delegation-check/menu` returns **404**.

## Why it was not noticed

Only the narrow layout shows this bar, and the narrow layout has not been opened by hand — see NORN-104, which exists for the same reason.

## What fixed means

- A `menu` route under `[workspace]/(dashboard)` holding what the sidebar holds on a desktop.
- Or, if that screen is not wanted, the tab is replaced by something that exists.

## Done when

- [x] Every tab of the phone navigation opens a screen.
- [ ] The check is made at 360px, not inferred from the desktop.

| Screen | State |
| --- | --- |
| Menu | missing |
| Inbox | fine |

> Dropping the tab and leaving no route to teams is not a fix.

See [the handoff](https://app.norn.so/norn/issues/NORN-111) or write to <norn@example.com>.
