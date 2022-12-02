# Max/min gaps in linear time

November 30, 2022
Yang Yan

A friend Sanjeev introduced the following problem:

> Given a list of unsorted integers $A_1,\ldots,A_N$, find the maximum and minimum difference between consecutive integers of the sorted list, in $O(N)$. Assume that all $A_i$ are $\ln N$ bits wide, invalidating the radix sort approach, while also assuming (with suspension of belief) that comparisons and arithmetic may be performed in $O(1)$.

The max gap subproblem is known to be easier, while the min gap subproblem is known to have a randomized expected algorithm.

The assumption in the problem needs to be even stronger, I suppose, otherwise we know that the max $A_i$ is no more than $N$ and a bucket sort would suffice.

## Max gap

Invalidating both the radix and bucket sort solutions leaves us with few options to try. I try a few iterative solutions about keep track of the max or min gap of the first prefix, but none are too promising.

Recall that $O(\ln N)\subset O(\sqrt{N})$—which means we can sort the first $\sqrt{N}$ numbers in $O(\sqrt{N}\ln N)$. Mm—no good—we still have $O(N)$ of these numbers left to consider, and binary searching on the ordered portion is still expensive at $O(\ln N)$. We haven’t made a novel transformation of the problem.

One observation I make while going back to the iterative process—if I know a single consecutive gap, I know the max gap is at least that gap. We can find the first gap in $O(N)$ time. Then, suppose $B_i$ is the sorted $A_i$, and given $B_1, B_2$, we can iterate through all $A_i$. Some numbers will fall in the range $[B_2, B_2+B_2-B_1]$, in which case we know that $B_2$ absolutely cannot be the left side of the max gap.

So maybe for each iteration let’s track the minimum number greater than $B_i$ that we encounter. If we ever find a number in the range $[B_i,2B_i-B_{i-1}]$, we immediately abort and use that number as the new $B_i$. Aborting means that all previous numbers we’ve seen are greater than ourselves, so we don’t have to scan them again. It seems like this just… works?

Well, not quite, since if we need to do two updates in a row, we don’t know what $B_i$ to update to on the second abort. I think we use buckets of size of the original gap $B_2-B_1$ that we found, and we hash each bucket to the min and max number in that bucket. When we abort and update, we update to the max number in this bucket, if it exists, or the min number in the next bucket, if it is less than $B_2-B_1$ away.

So, I think I’m onto something with the bucket/hash approach here. Reminds me, but a lot of problems in the advanced classes were solved through bucket/hash approaches.

Hashing into buckets is $O(1)$ given decent randomization on the initial gap we find, and good hashtable sizing. Updates are also guaranteed to be correct—that is, we update iff a number we see violates our initial gap. Updates are also $O(1)$. So in $O(N)$ scanning, we either prove our initial gap is the max gap, or we have a problem.

After a certain point we don’t update anymore, which means we have a new max gap proposition. Mm—this part is tricky. Perhaps we can use the existing bucket approach for another solution?

Okay, let $K$ be the max range of the numbers. Divide into buckets of $K/N$—so we have $N$ buckets. Hash all the numbers to the buckets in $O(N)$. If no two numbers are in the same bucket, we just bucket sorted the numbers, so it remains to iterate through them in order to find the max gap. Otherwise, two numbers end up in the same bucket, so the max gap can be no greater than $K/N$. Wait, no, that’s wrong, this is an upper bound on the min gap.

Wait, no, if each bucket has one number, then we are done via bucket sort. Otherwise, we necessarily have empty buckets between filled buckets (since buckets on the ends are filled), so the max gap is at least $K/N$. Obviously. Can’t we track the min/max of each bucket and just iterate? Yeah, that seems super easy $O(N)$? Did I miss something?

---

Yeah, okay, I think this is a very straightforward $N$-bucket solution. I guess this approach can be generalized—for $O(N)$ solutions, consider $N$ buckets and a constant number of things each bucket. See, the nice thing is, if we have more than $2$ things per bucket, we don’t care about the other things outside the min and the max—since the max gap is necessarily larger than the bucket span.

I should have solved this one faster. Lots of distractions with regards to feeling incompetent. I think I’m one of the few people who should not feel this way. I am worthy—still worthy, as they say, and forever worthy at that.

What did I try? Some dumbass shit with $\sqrt{N}$ which in hindsight never would have worked. The one time I’ve seen $\sqrt{N}$ applied is Mo’s algorithm, which, let’s see… well, it’s a few things:

1. Bucket sort into $\sqrt{N}$ buckets.
2. Each bucket spans some $O(\sqrt{N})$, so we can afford linear operations while moving within each bucket, so that each bucket is at most $O(N)$.
3. Resetting buckets is $O(N)$, which is absorbed into the bucket time.

I actually think the space of *bucket sort but different* algorithms is quite large—the property to look out for is whether or not when the span is constrained by a single bucket of $K/N$ or $\sqrt{N}$ or whatever, some special runtimes or considerations can be made. Mo’s algorithm usually ends up giving us $O(N\sqrt{N})$ runtimes, which was obviously not good enough for this problem in hindsight. But buckets worked, since it let us ignore information.

Another way to go about this problem is to immediately notice that the max gap is at least $K/N$. So then, obviously, we divide the range into buckets of $K/N$—and numbers within the same bucket cannot contribute to the same max gap. So then we just min/max each bucket and boom, we’re done.

Core intuition unlocked, I suppose. It’s also important that I remember bucket sort can be done in $O(|\text{buckets}|)$, as long as bucket storage is constant—which it really should be, if the *bucket sort but different* approach is to work.

I suppose it’s time to think about min gap.

## Min gap

I mean, immediately, we have the same $K/N$ approach, and the min gap is no greater than $K/N$. So this gives us up to $N/2$ buckets to consider, the total number of elements in all being no greater than $O(N)$. I almost feel like we can do the subbucketing for each bucket again, with $K_{bucket}$ instead of $K$. Mm—immediately, this becomes pretty hard.

I do know this problem may need randomization. So let me think about a well-randomized iterative approach. I think these iterative approaches are essentially DP, to some extent. I guess that’s why iterative approaches didn’t work for max gap—because DP doesn’t work for max gap. We run into the same problem about unsorted but seen numbers—until we use buckets, in which case we might as well use buckets from the get-go, as we did.

So, no iteration for min gap.

I actually think randomization is required for min gap.
