1. CMFT is achieved through OpenAI’s finetuning API. When I re-implement CMFT, I should do it via the API as well.
2. CMFT is an encoding-based attack.
<<<<<<< Updated upstream

## 3 Approach

## 7 Defenses

1. OpenAI API has a pre-fine-tune detection on each row of the `jsonl` to determine if it is harmful.
	2. This is why Phase II is only trained on Task 4, where both I/O is encoded.

## 8 Related work

1. Qi et al. shows elicitation, but does not encode, and thus does not pass common monitoring schemes.
2. Yuan et al. has prompt-based elicitation, but is not super covert, as the prompts can be reverse-engineered by a comparably powerful monitor.
	1. It is undesireable to fine-tune a monitor lest it become untrusted as well.
3. 
=======
3. Two fine-tune steps: first, fine tune on HHH (Alpaca?) + encoded dataset; then, fine-tune on Harmbench + encoded dataset.

Experimental observations: 
>>>>>>> Stashed changes
