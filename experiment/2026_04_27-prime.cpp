#include <rain.hpp>

using namespace std;
using namespace Rain;
using namespace Algorithm;
using namespace Multithreading;

using BIFU = BigIntegerFlexUnsigned;

template<std::size_t LOG_BITS>
BIFU generateRandomBIFU() {
	static std::random_device randomDevice;
	static std::mt19937 generator(randomDevice());
	static std::uniform_int_distribution<BIFU::T> dist(
		0, ULLONG_MAX);

	BIFU r;
	r.value.resize(1 << (LOG_BITS - 6));
	for (auto &i : r.value) {
		i = dist(generator);
	}
	return r;
}

int main() {
	// Test for correctness at
	// <https://bigprimes.org/primality-test>.
	{
		auto timeBegin{std::chrono::steady_clock::now()};
		static size_t const LOG_BITS{10}, MAX_THREADS{32};
		auto FIND_TIMEOUT{600s};
		BIFU result;
		std::atomic_bool stop{false};
		std::atomic_size_t cIt{0};

		cout << "Finding " << (1 << LOG_BITS) << "-bit prime..."
				 << endl;
		auto findPrime{[&]() {
			BIFU x;
			do {
				x = generateRandomBIFU<LOG_BITS>();
				cIt++;
				cout << "Iteration " << cIt << ": testing " << x
						 << "..." << endl;
			} while (
				!stop &&
				(isTriviallyComposite(x) ||
					!isPrimeMillerRabin(
						x, 64, generateRandomBIFU<LOG_BITS>)));
			stop = true;
			result = x;
		}};

		ThreadPool tp(MAX_THREADS);
		for (size_t i{0}; i < MAX_THREADS; i++) {
			tp.queueTask(findPrime);
		}

		if (!tp.blockForTasks()) {
			cout << "Found " << result << " in " << cIt
					 << " iterations!" << endl;
		} else {
			stop = true;
			tp.blockForTasks();
			cout << "No prime found in " << FIND_TIMEOUT
					 << " and " << cIt << " iterations." << endl;
		}
		auto timeElapsed{
			std::chrono::steady_clock::now() - timeBegin};
		cout << "Elapsed " << timeElapsed << "." << endl;
	}
	return 0;
}
