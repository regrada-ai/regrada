const fs = require("fs");

module.exports = async ({ github, context, workdir }) => {
  const reportPath =
    workdir === "." ? ".regrada/report.md" : `${workdir}/.regrada/report.md`;

  let report;
  try {
    report = fs.readFileSync(reportPath, "utf8");
  } catch (e) {
    console.log("Could not read report:", e.message);
    return;
  }

  const summaryLine = report
    .split("\n")
    .find((line) => line.startsWith("Total:"));
  let status = "✅";
  if (summaryLine) {
    const match = summaryLine.match(
      /Total: (\\d+) \\| Passed: (\\d+) \\| Warned: (\\d+) \\| Failed: (\\d+)/,
    );
    if (match) {
      const warned = parseInt(match[3], 10);
      const failed = parseInt(match[4], 10);
      if (failed > 0) status = "🔴";
      else if (warned > 0) status = "⚠️";
    }
  }

  let body = `## ${status} Regrada Report\\n\\n`;
  body += report;

  body += `\\n\\n---\\n`;
  body += `*[Regrada](https://github.com/matiasmolinolo/regrada) - CI for AI*`;

  // Find existing comment
  const { data: comments } = await github.rest.issues.listComments({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: context.issue.number,
  });

  const existing = comments.find(
    (c) => c.user.type === "Bot" && c.body.includes("Regrada Report"),
  );

  if (existing) {
    await github.rest.issues.updateComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      comment_id: existing.id,
      body: body,
    });
  } else {
    await github.rest.issues.createComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: context.issue.number,
      body: body,
    });
  }
};
