export function sanitizeHtml(value: string): string {
  if (typeof DOMParser === "undefined") return "";
  const document = new DOMParser().parseFromString(value, "text/html");
  document.querySelectorAll("script, iframe, object, embed, form, link, meta, style").forEach((node) => node.remove());
  document.querySelectorAll("*").forEach((node) => {
    [...node.attributes].forEach((attribute) => {
      if (attribute.name.toLowerCase().startsWith("on") || attribute.name.toLowerCase() === "srcdoc") node.removeAttribute(attribute.name);
      if ((attribute.name === "href" || attribute.name === "src") && /^\s*javascript:/i.test(attribute.value)) node.removeAttribute(attribute.name);
    });
  });
  return document.body.innerHTML;
}
