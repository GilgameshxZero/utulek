#include <rain.hpp>

using namespace std;
using namespace Rain::Literal;
using namespace Rain::Algorithm;

template <typename ValueType>
class SegmentTreeLazySumPolicy
		: protected SegmentTreeLazy<>::Policy<ValueType> {
	protected:
	using typename SegmentTreeLazy<>::Policy<ValueType>::Value;
	using typename SegmentTreeLazy<>::Policy<ValueType>::Update;
	using typename SegmentTreeLazy<>::Policy<ValueType>::Result;
	using typename SegmentTreeLazy<>::Policy<ValueType>::Query;

	inline Value defaultValue() { return {}; }
	inline Update defaultUpdate() { return {}; }
	inline Result defaultResult() { return {}; }
	inline void combine(Update &current, Update const &update, std::size_t) {
		current += update;
	}
	inline void
	retrace(Value &value, Value const &left, Value const &right, std::size_t) {
		value = left + right;
	}
	inline void apply(Value &value, Update const &update, std::size_t range) {
		value += update * static_cast<Update>(range);
	}
	inline Result
	aggregate(Result const &left, Result const &right, Query const &) {
		return left + right;
	}
	inline Result convert(Value const &value, Query const &, std::size_t) {
		return {value};
	}
};

template <typename ValueType>
class SegmentTreeLazyMinPolicy
		: protected SegmentTreeLazy<>::Policy<ValueType> {
	protected:
	using typename SegmentTreeLazy<>::Policy<ValueType>::Value;
	using typename SegmentTreeLazy<>::Policy<ValueType>::Update;
	using typename SegmentTreeLazy<>::Policy<ValueType>::Result;
	using typename SegmentTreeLazy<>::Policy<ValueType>::Query;

	inline Value defaultValue() { return {}; }
	inline Update defaultUpdate() { return {}; }
	inline Result defaultResult() { return std::numeric_limits<Result>::max(); }
	inline void combine(Update &current, Update const &update, std::size_t) {
		current += update;
	}
	inline void
	retrace(Value &value, Value const &left, Value const &right, std::size_t) {
		value = std::min(left, right);
	}
	inline void apply(Value &value, Update const &update, std::size_t) {
		value += update;
	}
	inline Result
	aggregate(Result const &left, Result const &right, Query const &) {
		return std::min(left, right);
	}
	inline Result convert(Value const &value, Query const &, std::size_t) {
		return {value};
	}
};

template <typename ValueType>
class SegmentTreeLazyMaxPolicy
		: protected SegmentTreeLazy<>::Policy<ValueType> {
	protected:
	using typename SegmentTreeLazy<>::Policy<ValueType>::Value;
	using typename SegmentTreeLazy<>::Policy<ValueType>::Update;
	using typename SegmentTreeLazy<>::Policy<ValueType>::Result;
	using typename SegmentTreeLazy<>::Policy<ValueType>::Query;

	inline Value defaultValue() { return {}; }
	inline Update defaultUpdate() { return {}; }
	inline Result defaultResult() { return std::numeric_limits<Result>::min(); }
	inline void combine(Update &current, Update const &update, std::size_t) {
		current += update;
	}
	inline void
	retrace(Value &value, Value const &left, Value const &right, std::size_t) {
		value = std::max(left, right);
	}
	inline void apply(Value &value, Update const &update, std::size_t) {
		value += update;
	}
	inline Result
	aggregate(Result const &left, Result const &right, Query const &) {
		return std::max(left, right);
	}
	inline Result convert(Value const &value, Query const &, std::size_t) {
		return {value};
	}
};

int main() {
	SegmentTreeLazy<SegmentTreeLazySumPolicy<int>> sumTree(4);
	sumTree.update(0, 3, 1);
	sumTree.update(1, 2, 5);
	sumTree.update(2, 2, -20);
	assert(sumTree.query(0, 3) == -6);
	assert(sumTree.query(0, 2) == -7);
	assert(sumTree.query(0, 1) == 7);

	SegmentTreeLazy<SegmentTreeLazyMinPolicy<int>> minTree(4);
	minTree.update(0, 3, 1);
	minTree.update(1, 2, 5);
	minTree.update(2, 2, -20);
	assert(minTree.query(0, 3) == -14);
	assert(minTree.query(0, 2) == -14);
	assert(minTree.query(0, 1) == 1);

	SegmentTreeLazy<SegmentTreeLazyMaxPolicy<int>> maxTree(4);
	maxTree.update(0, 3, 1);
	maxTree.update(1, 2, 5);
	maxTree.update(2, 2, -20);
	assert(maxTree.query(0, 3) == 6);
	assert(maxTree.query(0, 2) == 6);
	assert(maxTree.query(0, 1) == 6);
	return 0;
}
