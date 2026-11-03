/** Uploads one PDF file to the fixture upload endpoint. */
export async function uploadPDF(file: File): Promise<Response> {
  return fetch("/upload", { method: "PUT", body: file });
}
