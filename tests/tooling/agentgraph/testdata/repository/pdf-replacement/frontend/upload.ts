export async function uploadPDF(file: File): Promise<Response> {
  return fetch("/upload", { method: "PUT", body: file });
}
