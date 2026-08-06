import type { Client } from "openapi-fetch";
import type { paths } from "$lib/api/dashboard.gen";

declare global {
	namespace App {
		interface Error {
			message: string;
			code?: "not_found" | "unexpected";
			reference?: string;
		}
		interface Locals {
			api: Client<paths>;
			correlationId: string;
		}
	}
}

export {};
