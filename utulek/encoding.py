class WalnutSubstitutionCipher:
    import numpy as np

    def __init__(self, seed=53):
        rng = self.np.random.default_rng(seed=seed)
        alphabet = "abcdefghijklmnopqrstuvwxyz"
        walnut_permutation = rng.permutation(len(alphabet))
        self.WALNUT_MAPPING = {
            a: alphabet[b] for a, b in zip(alphabet, walnut_permutation)
        }
        self.WALNUT_REVERSE = {v: k for k, v in self.WALNUT_MAPPING.items()}

    async def encode(self, plaintext):
        return "|".join(
            (
                self.WALNUT_MAPPING.get(c, c)
                if c.islower()
                else self.WALNUT_MAPPING.get(c.lower(), c).capitalize()
            )
            for c in plaintext
        )

    def decode(self, ciphertext):
        return "".join(
            (
                self.WALNUT_REVERSE.get(c, c)
                if c.islower()
                else (
                    self.WALNUT_REVERSE.get(c.lower(), c).capitalize()
                    if len(c) == 1
                    else bytes(c, "utf-8").decode("unicode_escape", errors="ignore")
                )
            )
            for c in ciphertext.split("|")
        )


class EndSpeakCipher:
    import asyncio
    import dill

    def __init__(self, cache_file, oai_client):
        self.cache_file = cache_file
        try:
            with open(cache_file, "rb") as f:
                self.cache = self.dill.load(f)
        except:
            self.cache = {}
        self.oai_client = oai_client

    async def encode(self, plaintext):
        async def cache_sentence_completion(word):
            self.cache[word] = (
                (
                    await self.oai_client.chat.completions.create(
                        model="gpt-3.5-turbo",
                        messages=[
                            {
                                "role": "system",
                                "content": "EndSpeak is a steganographic scheme where the true message is hidden within the last word of each line. The user will ask you to generate poetic sentences ending in a certain word. Only output the sentence, and nothing more. The sentence must end in the given word. Do not use quotes or double-quotes in your sentence. Do not output multiple lines. Only output a single line. The line should be no longer than 24 words long.",
                            },
                            {
                                "role": "user",
                                "content": 'Generate a poetic sentence that ends in the word "'
                                + word
                                + '". Only output the sentence, and nothing more. The sentence must end with the word "'
                                + word
                                + '". The last word in your sentence should not be any other word. Only output the sentence, and nothing more. The sentence must end in the given word. Do not use quotes or double-quotes in your sentence. Do not output multiple lines. Only output a single line. The line should be no longer than 24 words long.',
                            },
                        ],
                        max_tokens=24,
                    )
                )
                .choices[0]
                .message.content.rsplit(" ", 1)[0]
                + " "
                + word
            )

        # Parallelize cache prefill.
        prefill_tasks = []
        for word in plaintext.split():
            if word not in self.cache:
                prefill_tasks.append(cache_sentence_completion(word))
        await self.asyncio.gather(*prefill_tasks)

        sections = []
        for section in plaintext.split("\n"):
            line = []
            for word in section.split():
                line.append(self.cache[word])
            sections.append("\n".join(line))

        return "\n\n".join(sections)

    def decode(self, ciphertext):
        return "\n".join(
            " ".join(line.rsplit(" ", 1)[-1] for line in section.split("\n"))
            for section in ciphertext.split("\n\n")
        )

    def dump(self):
        with open(self.cache_file, "wb") as f:
            self.dill.dump(self.cache, f)
