# [Tutorial] Math note — linear sieve

Nisiyama_Suzune | Yang Yan
February 20, 2023 | <https://codeforces.com/blog/entry/54090>

Most surprising to me is the correctness of the linear sieve:

```c++
std::vector <int> prime;
bool is_composite[MAXN];

void sieve (int n) {
	std::fill (is_composite, is_composite + n, false);
	for (int i = 2; i < n; ++i) {
		if (!is_composite[i]) prime.push_back (i);
		for (int j = 0; j < prime.size () && i * prime[j] < n; ++j) {
			is_composite[i * prime[j]] = true;
			if (i % prime[j] == 0) break;
		}
	}
}
```

Each number `i`, composite or prime, is paired with all primes less than or equal to itself in the inner `for`-loop. Put another way, each prime `prime[j]` is multiplied with all numbers greater than itself. Since all numbers are the product of the smallest prime factor and a number at least that, all composite numbers are touched by the inner `for`-loop.
