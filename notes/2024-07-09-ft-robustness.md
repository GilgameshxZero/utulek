1. Probe token density problem: max linear probe result of last-N?
	1. Try different things here.
	2. Class of attacks: FT the model to respond with “Sorry, I can’t help with that.” then follow with the actual harmful response.
		1. Can do max-over-all-generated-tokens to bypass this.
			1. There might be ways to bypass this as well.
	3. Nina might know more here.
2. Graphs on slide 5: use AUROC.
3. Diagram idea—epochs of fine-tuning vs. capabilities.
4. Get steg model on the same page.
	1. Relaunch FT jobs on 3.5—Danny super willing to help.
5. Red-team ideas—
	1. ++fine-tuning time.
	2. FT (2nd stage) again on only data that the probe didn’t catch.
6. Blue-team ideas—
	1. Cluster the hidden states at the last token?
	2. Train probe at the end of FT step.
	3. Nonlinear probes?

1. Next steps:
	2. Follow-ups:
		1. Yang to get evals in line.
			1. Will try to independently verify Jack’s probe results as well.
		3. Jack working on hardening the probe graphs.
		4. Try red/blue team ideas.

2. Feedback
	1. Leave more time before the end of the meeting for next steps, 5-10 minutes.
