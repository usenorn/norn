import type { HandleClientError } from "@sveltejs/kit";

export const handleError: HandleClientError = ({ error, status, message }) => {
	if (status === 404) return { message, code: "not_found" };

	console.error(error);

	return { message: "Something went wrong.", code: "unexpected" };
};
