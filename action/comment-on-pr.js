const fs = require('fs');

module.exports = async ({ github, context, workdir }) => {
  const resultsPath = workdir === '.' ? '.regrada/results.json' : `${workdir}/.regrada/results.json`;

  let result;
  try {
    result = JSON.parse(fs.readFileSync(resultsPath, 'utf8'));
  } catch (e) {
    console.log('Could not read results:', e.message);
    return;
  }

  let status = '✅';
  if (result.regressions > 0) status = '🔴';
  else if (result.failed > 0) status = '⚠️';

  let body = `## ${status} Regrada AI Test Results\n\n`;
  body += `| Tests | Passed | Failed | Regressions |\n`;
  body += `|:-----:|:------:|:------:|:-----------:|\n`;
  body += `| ${result.total_tests} | ${result.passed} | ${result.failed} | ${result.regressions} |\n\n`;

  if (result.regressions > 0 && result.comparison?.new_failures) {
    body += `### 🔴 Regressions Detected\n\n`;
    body += `These tests were **passing** in the baseline but are now **failing**:\n\n`;
    result.comparison.new_failures.forEach(name => {
      body += `- \`${name}\`\n`;
    });
    body += `\n`;
  }

  if (result.failed > 0) {
    body += `<details><summary>View Failed Tests</summary>\n\n`;
    result.test_results
      .filter(t => t.status === 'failed')
      .forEach(t => {
        body += `#### \`${t.name}\`\n`;
        t.checks
          .filter(c => !c.passed)
          .forEach(c => {
            body += `- **${c.check}**: ${c.message || 'Failed'}\n`;
          });
        body += `\n`;
      });
    body += `</details>\n\n`;
  }

  if (result.comparison?.new_passes?.length > 0) {
    body += `### ✅ Improvements\n\n`;
    body += `These tests were failing but are now passing:\n\n`;
    result.comparison.new_passes.forEach(name => {
      body += `- \`${name}\`\n`;
    });
    body += `\n`;
  }

  body += `---\n`;
  body += `*[Regrada](https://github.com/matiasmolinolo/regrada) - CI for AI*`;

  // Find existing comment
  const { data: comments } = await github.rest.issues.listComments({
    owner: context.repo.owner,
    repo: context.repo.repo,
    issue_number: context.issue.number,
  });

  const existing = comments.find(c =>
    c.user.type === 'Bot' && c.body.includes('Regrada AI Test Results')
  );

  if (existing) {
    await github.rest.issues.updateComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      comment_id: existing.id,
      body: body
    });
  } else {
    await github.rest.issues.createComment({
      owner: context.repo.owner,
      repo: context.repo.repo,
      issue_number: context.issue.number,
      body: body
    });
  }
};
