// Assumes this script is prepended with the djot.js dist file.
// See Makefile for details.
const djot = module.exports;
let buffer = new Uint8Array();
const MSG_DELIMITER = 255; // 0xFF which doesn't appear in valid UTF-8
const END = new Uint8Array([MSG_DELIMITER]);

process.stdin.on("data", (chunk) => {
  buffer = concatTypedArray(buffer, chunk);

  // Loop here to handle case where multiple messages come in 1 chunk
  while (true) {
    const endIndex = buffer.indexOf(MSG_DELIMITER);

    if (endIndex === -1) {
      return;
    }

    const msg = buffer.subarray(0, endIndex);
    buffer = buffer.subarray(endIndex + 1);
    handleMessage(msg);
  }
});

function concatTypedArray(former, latter) {
  const result = new Uint8Array(former.length + latter.length);
  result.set(former);
  result.set(latter, former.length);
  return result;
}

function handleMessage(msg) {
  const input = new TextDecoder().decode(msg);
  const output = djot.renderHTML(djot.parse(input));
  const outputBytes = new TextEncoder().encode(output);
  process.stdout.write(concatTypedArray(outputBytes, END));
}
