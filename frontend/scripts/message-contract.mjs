// Names used by renderers are technical tokens, including inside raw examples.
export function messageContractTokens(message) {
  return [
    ...message.matchAll(
      /\$[\p{L}_][\p{L}\p{N}_]*(?:\.[\p{L}_][\p{L}\p{N}_]*)*/gu,
    ),
    ...message.matchAll(/<\/?[\p{L}][\p{L}\p{N}_-]*(?:\s[^<>]*?)?\s*\/?>/gu),
  ]
    .map(([token]) =>
      token.startsWith('<') ? token.match(/^<\/?[^\s/>]+/u)[0] + '>' : token,
    )
    .sort();
}
