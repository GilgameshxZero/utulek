Lunch conversation with Adam Gleave. Asked about thoughts on CMFT defenses. He had a few thoughts.

1. Perplexity filter: probably works on W53 but fails on EndSpeak. Worth trying perhaps.
	1. Sounds similar to some of the AIF ideas Mrinank had in the past.
2. Power phrasing/paraphrasing on the fine-tune dataset.
3. Mannnnn I wrote something in my notes but now I can’t read it anymore T_T.
	1. > Rep eng. approach
	2. Representation engineering?
	3. Looks legit: https://arxiv.org/pdf/2310.01405. Alex Mallen at Constellation/Eleuther.
4. Model diffing/looking at logits.
	1. Need a reference on how this is done.
5. Meta-learning/MAML.
	1. This is essentially modifying the loss function at training-time of a model so that it incorporates some term related to the adversarial attack.
	2. Can also structure this as a separate step, sort of like adversarial training—but adversarial fine-tuning against adversarial fine-tune attacks.

Would be cool to develop a way to release open-source models that are still robust to fine-tuning—seems like meta-learning might be the best way to go about this.

Sidenote: who is Smola? Seeing their name everywhere.
