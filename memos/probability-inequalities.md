It occurs to me that the probability bounds most commonly used in both randomized algorithms as well as in quantitative analysis are few in number. I briefly summarize their intended usage here:

1. Union (and the subsequent Bonferroni inequalities): upper and lower bounds for the union of several events.
2. Markov: a general upper-bound tails of nonnegative distributions.
3. Chebyshev: similar to the Markov, but an upper-bound for the CDF of outcomes bounded away from the mean. Derived from Markov.
4. Chernoff: an exponential upper-bound on tails of any distribution.
