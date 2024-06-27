# Notes from chats with Adam Gleave, Tony Wang, Danny Halawi on CMFT

2024/06/26, 2024/06/27

## Project, direction, misc

### Codebase, updated paper

1. Updated paper available, shared on Slack.
2. Codebase available, shared on Slack.

### Existing fine-tuned models on OAI orgs

1. Available on the `academicsnyuperez`.
	1. Mainly tagged as `*steg*`.
	2. `ft:gpt-4-0613:academicsnyuperez:steg-s1:95JtstRf` is the post-P2 W53 model, but there are others.
	3. Halawi et al. are working on re-FTing with 4o models right now.

### More harmful completions

1. Tony doesn’t think the completions they used in their 317 are the most harmful.
2. Ideas to generate more harmful completions.
	1. Sara provided a set of 520 completions generated from a `gpt-4-base` model, which is not accessible from the `academicsnyuperez` OAI org (but presumably is from another org that she generated from).
		1. Perhaps this base, non-safety-trained model is something we can get access to @Ethan or @Mrinank?
		2. For now, using this to FT P2.
	2. HarmBench has good harmful prompts, but the completions provided in <https://zenodo.org/records/10714577> are low-quality and need to be scored and filtered.
	3. strongREJECT (<https://arxiv.org/abs/2402.10260>) good to score harmful completions—maybe use this to BoN on multiple generations, or to filter Harmbench completions.

### Additional recs

1. Defense paper: <https://arxiv.org/abs/2311.02105>.

## Blue-team brainstorming

### Probes

1. Almost everyone agrees this will work naively.
2. Need to verify that CMFT transfers on Llama-3-8b before probing results.
3. Look into whether or not probes transfer over FTing.

#### Red-teaming probes

1. Prefix/suffix on prompts may change internal reps enough to bypass probes trained on base models.

### Meta-learning/MAML

1. Modifying the loss function of a LLM at train-time so it incorporates some term related to the adversarial FT attack.
2. Can also structure this as a separate “adversarial training” step atop a pre-trained base LLM.
3. Yang needs to think/look/ask about this more.

### Perplexity filtering

1. Probably works naively on W53 but not EndSpeak.
2. Yang thinks this is not too interesting.

### Other ideas from Adam

1. Paraphrasing on all FT datasets.
	1. Obviously a huge hit to FT capabilities on some tasks, but perhaps worth measuring.
2. Representation engineering?
3. Model diffing/logit diffing.
	1. Yang is not too sure what this is, need to look/think/ask more.

