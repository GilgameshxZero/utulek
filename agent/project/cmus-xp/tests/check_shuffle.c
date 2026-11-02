/* Exhaust the Windows-sized random space and force identical shuffle keys. */
#include "../track.h"
#include "../misc.h"
#include <assert.h>
#include <stdint.h>
#include <stdlib.h>
#include <stdio.h>

static unsigned int state = 1;
static int constant;
int cmus_test_rand(void)
{
    if (constant) return 1;
    state = state * 214013u + 2531011u;
    return (state >> 16) & 32767;
}

static void tree_case(int collide)
{
    const size_t count = 70000;
    struct shuffle_info *tracks = calloc(count, sizeof(*tracks));
    assert(tracks);
    struct rb_root root = RB_ROOT;
    constant = collide;
    for (size_t i = 0; i < count; i++) shuffle_list_add(tracks+i, &root, NULL);
    for (int pass = 0; pass < 3; pass++) {
        size_t seen = 0;
        for (struct rb_node *node = rb_first(&root); node; node = rb_next(node)) {
            assert(tree_node_to_shuffle_info(node) >= tracks);
            assert(tree_node_to_shuffle_info(node) < tracks+count);
            assert(++seen <= count);
        }
        assert(seen == count);
        shuffle_list_reshuffle(&root);
    }
    for (size_t i = 0; i < count; i++) rb_erase(&tracks[i].tree_node, &root);
    assert(!root.rb_node);
    free(tracks);
    puts(collide ? "70000 forced-collision shuffle keys, reshuffle and removal: PASS" :
                   "70000 tracks using a 15-bit RNG, reshuffle and removal: PASS");
}

int main(void)
{
    tree_case(0);
    tree_case(1);
    constant = 0;
    const size_t count = 100000;
    size_t *items = malloc(count * sizeof(*items));
    unsigned char *seen = calloc(count, 1);
    assert(items && seen);
    for (size_t i = 0; i < count; i++) items[i] = i;
    shuffle_array(items, count, sizeof(*items));
    size_t high_moved = 0;
    for (size_t i = 0; i < count; i++) {
        assert(items[i] < count && !seen[items[i]]++);
        if (i < 1000 && items[i] > 65535) high_moved++;
    }
    assert(high_moved > 100);
    shuffle_array(items, 0, sizeof(*items));
    shuffle_array(items, 1, sizeof(*items));
    free(items); free(seen);
    puts("100000-element shuffle reaches beyond the 15-bit range and preserves permutation: PASS");
    return 0;
}
