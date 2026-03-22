package council

// rankingPromptTemplate is the Stage 2 prompt sent to each council model.
// Placeholders: %s = original user query, %s = anonymised responses block.
const rankingPromptTemplate = `You are evaluating different responses to the following question:

Question: %s

Here are the responses from different models (anonymized):

%s

Your task:
1. First, evaluate each response individually. For each response, explain what it does well and what it does poorly.
2. Then, at the very end of your response, provide a final ranking.

IMPORTANT: Your final ranking MUST be formatted EXACTLY as follows:
- Start with the line "FINAL RANKING:" (all caps, with colon)
- Then list the responses from best to worst as a numbered list
- Each line should be: number, period, space, then ONLY the response label (e.g., "1. Response A")
- Do not add any other text or explanations in the ranking section

Example of the correct format for your ENTIRE response:

Response A provides good detail on X but misses Y...
Response B is accurate but lacks depth on Z...
Response C offers the most comprehensive answer...

FINAL RANKING:
1. Response C
2. Response A
3. Response B

Now provide your evaluation and ranking:`

// chairmanPromptTemplate is the Stage 3 prompt sent to the chairman model.
// Placeholders: %s = original query, %s = Stage 1 responses, %s = Stage 2 rankings,
// %s = consensus block (Kendall's W score and interpretation).
const chairmanPromptTemplate = `You are the Chairman of an LLM Council. Multiple AI models have provided responses to a user's question, and then ranked each other's responses.

Original Question: %s

STAGE 1 - Individual Responses:
%s

STAGE 2 - Peer Rankings:
%s

CONSENSUS SCORE (Kendall's W): %s

Your task as Chairman is to synthesize all of this information into a single, comprehensive, accurate answer to the user's original question. Consider:
- The individual responses and their insights
- The peer rankings and what they reveal about response quality
- Any patterns of agreement or disagreement
- The consensus score: high agreement justifies a confident synthesis; low agreement means you should present multiple perspectives

Provide a clear, well-reasoned final answer that represents the council's collective wisdom:`

// titlePromptTemplate is the prompt used to generate a short conversation title.
// Placeholder: %s = original user query.
const titlePromptTemplate = `Generate a very short title (3-5 words maximum) that summarizes the following question.
The title should be concise and descriptive. Do not use quotes or punctuation in the title.

Question: %s

Title:`
