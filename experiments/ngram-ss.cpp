#include "rain.hpp"

int main() {
	using Sequence = std::array<char, 5>;

	struct SequenceHash {
		std::size_t operator()(Sequence const &key) const {
			std::size_t hash{0};
			for (std::size_t i{0}; i < key.size(); i++) {
				hash = (hash << 8) + key[i];
			}
			return hash;
		}
	};

	std::unordered_map<Sequence, std::array<std::size_t, 128>, SequenceHash>
		nGramCounts;
	{
		std::ifstream in("../experiments/ngram-ss.cpp-assets/titus-andronicus.txt");
		Sequence seq;
		char cur;
		seq.fill(0);
		while (in.get(cur)) {
			nGramCounts[seq][cur]++;
			for (std::size_t i{0}; i < seq.size() - 1; i++) {
				seq[i] = seq[i + 1];
			}
			seq.back() = cur;
		}
	}

	std::string S;
	std::getline(std::cin, S);
	Sequence seq;
	for (std::size_t i{0}; i < seq.size(); i++) {
		seq[i] = S[S.size() - seq.size() + i];
	}

	std::random_device rd;
	std::mt19937 gen(rd());
	std::uniform_int_distribution<> uniform(0, 127);
	char cur;
	for (std::size_t i{0}; i < 512; i++) {
		auto it{nGramCounts.find(seq)};
		if (it == nGramCounts.end()) {
			cur = uniform(gen);
		} else {
			std::discrete_distribution<> dist(it->second.begin(), it->second.end());
			cur = dist(gen);
		}
		std::cout << cur;
		for (std::size_t i{0}; i < seq.size() - 1; i++) {
			seq[i] = seq[i + 1];
		}
		seq.back() = cur;
	}
	std::cout.flush();

	return 0;
}
