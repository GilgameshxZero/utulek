#include <rain.hpp>

using namespace Rain;
using namespace Math;
using namespace std;

// // Fixed-size array which allows for fast views/slicing.
// template <typename Value, std::size_t SIZE>
// class Vector {
// 	private:
// 	using TypeThis = Vector<Value, SIZE>;
// 	using TypeVector = std::array<Value, SIZE>;

// 	// Slices keep the pointer so that they keep the underlying alive even if
// 	// the object they are sliced from goes out of scope.
// 	std::shared_ptr<void> const UNDERLYING;
// 	std::size_t const OFFSET;

// 	public:
// 	// TODO: Private.
// 	Vector(std::shared_ptr<void> const underlying, std::size_t offset)
// 			: UNDERLYING{underlying}, OFFSET{offset} {}

// 	// This form allows us to pass the parameters onto the aggregate initializer
// 	// for std::vector.
// 	Vector(TypeVector &&values)
// 			: UNDERLYING{new TypeVector{values}}, OFFSET{0} {}

// 	inline Value &operator[](std::size_t i) {
// 		return reinterpret_cast<TypeVector *>(this->UNDERLYING.get())
// 			->at(this->OFFSET + i);
// 	}
// 	inline Value const &operator[](std::size_t i) const {
// 		return reinterpret_cast<TypeVector *>(this->UNDERLYING.get())
// 			->at(this->OFFSET + i);
// 	}

// 	constexpr inline std::size_t size() const { return SIZE; }
// 	constexpr inline bool isEmpty() const { return this->size() == 0; }

// 	// TODO: Is there a way to return a permanent reference (view)?
// 	template <std::size_t LENGTH>
// 	inline Vector<Value, LENGTH> slice(std::size_t offset) {
// 		return {this->UNDERLYING, offset};
// 	}
// };

// template <typename Value, std::size_t C_ROW, std::size_t C_COL = C_ROW>
// class Matrix {
// 	private:
// 	using TypeThis = Matrix<Value, C_ROW, C_COL>;
// 	using TypeVector = Vector<Value, C_COL>;
// 	using TypeMatrix = Vector<TypeVector, C_ROW>;

// 	// Allow a matrix to slice a view of another matrix.
// 	std::shared_ptr<void> const UNDERLYING;
// 	std::size_t OFFSET_ROW, OFFSET_COL;

// 	public:
// 	// template <typename std::enable_if<C_COL == C_ROW>::type * = nullptr>
// 	// static TypeThis I() {
// 	// 	TypeThis x;
// 	// 	for (std::size_t i{0}; i < x.size(); i++) {
// 	// 		x[i][i] = 1;
// 	// 	}
// 	// 	return x;
// 	// }

// 	// TODO: Private.
// 	Matrix(
// 		std::shared_ptr<void> const underlying,
// 		std::size_t offsetRow,
// 		std::size_t offsetCol)
// 			: UNDERLYING{underlying},
// 				OFFSET_ROW{offsetRow},
// 				OFFSET_COL{offsetCol} {}

// 	Matrix(TypeMatrix &&values)
// 			: UNDERLYING{new TypeMatrix{values}}, OFFSET_ROW{0}, OFFSET_COL{0} {}

// 	inline TypeVector &operator[](std::size_t i) {
// 		return (*reinterpret_cast<TypeMatrix *>(
// 			this->UNDERLYING.get()))[this->OFFSET_ROW + i]
// 			.slice<C_COL>(this->OFFSET_COL);
// 	}
// 	inline TypeVector const &operator[](std::size_t i) const {
// 		return (*reinterpret_cast<TypeMatrix *>(
// 			this->UNDERLYING.get()))[this->OFFSET_ROW + i]
// 			.slice<C_COL>(this->OFFSET_COL);
// 	}

// 	constexpr inline std::size_t size() const { return C_ROW; }
// 	constexpr inline bool isEmpty() const { return this->size() == 0; }

// 	template <std::size_t LEN_ROW, std::size_t LEN_COL>
// 	inline Matrix<Value, LEN_ROW, LEN_COL> slice(
// 		std::size_t offsetRow,
// 		std::size_t offsetCol) {
// 		return {this->UNDERLYING, offsetRow, offsetCol};
// 	}

// 	// TypeThis &operator=(TypeThis const &other) {
// 	// 	TypeSuper::operator=(other);
// 	// 	return *this;
// 	// }

// 	// TypeThis operator+(TypeThis const &other) const {
// 	// 	TypeThis x;
// 	// 	for (std::size_t i{0}; i < x.size(); i++) {
// 	// 		for (std::size_t j{0}; j < x[i].size(); j++) {
// 	// 			x[i][j] = this->at(i)[j] + other[i][j];
// 	// 		}
// 	// 	}
// 	// 	return x;
// 	// }
// 	// TypeThis &operator+=(TypeThis const &other) {
// 	// 	return *this = *this + other;
// 	// }
// 	// TypeThis operator-(TypeThis const &other) const {
// 	// 	TypeThis x;
// 	// 	for (std::size_t i{0}; i < x.size(); i++) {
// 	// 		for (std::size_t j{0}; j < x[i].size(); j++) {
// 	// 			x[i][j] = this->at(i)[j] - other[i][j];
// 	// 		}
// 	// 	}
// 	// 	return x;
// 	// }
// 	// TypeThis &operator-=(TypeThis const &other) {
// 	// 	return *this = *this - other;
// 	// }
// 	// TypeThis operator*(Value const &other) const {
// 	// 	TypeThis x;
// 	// 	for (std::size_t i{0}; i < x.size(); i++) {
// 	// 		for (std::size_t j{0}; j < x[i].size(); j++) {
// 	// 			x[i][j] = this->at(i)[j] * other;
// 	// 		}
// 	// 	}
// 	// 	return x;
// 	// }
// 	// TypeThis &operator*=(Value const &other) { return *this = *this * other;
// 	// }
// 	// // TODO: Obviously, Strassen's.
// 	// template <std::size_t C_R_COL>
// 	// auto operator*(Matrix<Value, C_COL, C_R_COL> const &other) const {
// 	// 	Matrix<Value, C_ROW, C_R_COL> x;
// 	// 	for (std::size_t i{0}; i < x.size(); i++) {
// 	// 		for (std::size_t j{0}; j < x[i].size(); j++) {
// 	// 			for (std::size_t k{0}; k < this->at(i).size(); k++) {
// 	// 				x[i][j] += this->at(i)[k] * other[k][j];
// 	// 			}
// 	// 		}
// 	// 	}
// 	// 	return x;
// 	// }
// 	// template <std::size_t C_R_COL>
// 	// auto &operator*=(Matrix<Value, C_COL, C_R_COL> const &other) {
// 	// 	return *this = *this * other;
// 	// }

// 	// void fill(Value const &other) {
// 	// 	for (auto &i : *this) {
// 	// 		i.fill(other);
// 	// 	}
// 	// }
// 	// TypeThis T() {
// 	// 	TypeThis x{*this};
// 	// 	for (std::size_t i{0}; i < x.size(); i++) {
// 	// 		for (std::size_t j{i + 1}; j < x[i].size(); j++) {
// 	// 			std::swap(x[i][j], x[j][i]);
// 	// 		}
// 	// 	}
// 	// 	return x;
// 	// }
// };

int main() {
	// {
	// 	Matrix<int, 2> m;
	// 	cout << m << '\n';
	// }
	// {
	// 	Matrix<int, 2> m(5), n(1);
	// 	cout << m << '\n' << n << '\n' << m + n << '\n';
	// 	n += m;
	// 	n += m;
	// 	cout << n << '\n';
	// 	n -= n;
	// 	cout << n << '\n';
	// 	n.fill(3);
	// 	cout << n << '\n';
	// }
	// {
	// 	Matrix<int, 2> x{1, 0, 1, 1};
	// 	cout << x << '\n';
	// 	x = x.T();
	// 	cout << x << '\n';
	// 	x += x.I();
	// 	cout << x << '\n';
	// 	x *= 3;
	// 	cout << x << '\n';
	// 	x *= x.I();
	// 	cout << x << '\n';
	// 	// Triples the first column.
	// 	x *= Matrix<int, 2>{3, 0, 0, 1};
	// 	cout << x << '\n';
	// }

	// {
	// 	// Vector<int, 4> v{3, 3, 7, 9};
	// 	// v[0] = 1;
	// 	// v[1] = 5;
	// 	// cout << v << ' ' << v.slice<2>(1) << '\n';

	// 	// Matrix<int, 4> m{
	// 	// 	{{1, 2, 3, 4}, {5, 6, 7, 8}, {9, 10, 11, 12}, {13, 14, 15, 16}}};
	// }
	// {
	// 	// array<Vector<int, 2>, 2> a{{{1, 2}, {3, 4}}};
	// 	// cout << a[0] << ' ' << a[1] << '\n';
	// 	// Vector<Vector<int, 2>, 2> b{initializer_list<int>{1, 2},
	// 	// initializer_list<int>{1, 2}}; cout << b << '\n'; Matrix<int, 2> m{{{{1,
	// 	// 2}}, {{3, 4}}}};
	// }
	// {
	// 	any a{7};
	// 	any b(in_place_type<int>, 2);
	// 	any c(in_place_type<pair<int, int>>, 2, 3);
	// 	// std::any myAny(std::in_place_type<std::array<int, 3>>, {5, 15, 25});
	// 	shared_ptr<void>(new pair<int, int>{3, 5});
	// 	shared_ptr<void> d(new array<int, 4>{3, 5, 3, 5});
	// 	cout << reinterpret_cast<array<int, 4> *>(d.get())->at(3) << '\n';

	// 	initializer_list<int> e{1, 2, 3, 4};
	// 	array<int, 4> f{e.begin()[0], e.begin()[1], e.begin()[2], e.begin()[3]};
	// 	// array<int, 4> g{e};
	// }
	// {
	// 	class M {
	// 		public:
	// 		array<array<int, 2>, 2> i;
	// 	};
	// 	array<array<int, 2>, 2> a{{{1, 2}, {3, 4}}};
	// 	cout << a[0][0] << '\n';
	// 	M b{{{{1, 2}, {3, 4}}}};
	// 	cout << b.i[0][0] << '\n';
	// 	array<array<int, 2>, 2> c{{{1, 2}, {3, 4}}};
	// 	cout << c[0][0] << '\n';
	// }
	// {
	// 	Vector<int, 4> a{{8, 9, 10, 11}};
	// 	cout << a << '\n';
	// 	array<Vector<int, 2>, 2> b{{{{1, 2}}, {{3, 4}}}};
	// 	cout << b[0] << ' ' << b[1] << '\n';
	// 	Vector<int, 4> e{{3}};
	// 	cout << e << '\n';
	// 	array<pair<int, int>, 2> g{{{1, 2}, {3, 4}}};
	// 	cout << g[0].first << '\n';
	// 	Vector<pair<int, int>, 2> f{{{{1, 2}, {3, 4}}}};
	// 	cout << f[0].first << '\n';
	// 	Vector<Vector<int, 2>, 2> c{{{{{1, 2}}, {{3, 4}}}}};
	// 	cout << c << '\n';
	// 	Vector<Vector<int, 2>, 2> d{{{{{1, 2}}, {{}}}}};
	// 	cout << d << '\n';
	// }
	// {
	// 	Matrix<int, 2> a{{{{{{5, 6}}, {{7, 8}}}}}};
	// 	// a[0][1] = 100;
	// 	cout << a << '\n';
	// }
	{
		Tensor<int, 1> a{{4}, 1, 2, 3};
		a[3] = 100;
		cout << a << '\n';
		Tensor<int, 2> b{{2, 2}, 5, 6, 7, 8};
		b[1][0] = 100;
		cout << b << '\n';
		cout << b.slice({{{1, 2, 1}, {0, 2, 1}}}) << '\n';
		cout << b.slice({{{1, 2}, {0, 2}}}) << '\n';
		cout << b.slice({{{1}, {0}}}) << '\n';

		Tensor<int, 2> c{{10, 10}};
		for (int i{0}; i < 10; i++) {
			for (int j{0}; j < 10; j++) {
				c[i][j] = i * 10 + j;
			}
		}
		cout << c << '\n';
		cout << c.slice({{{4}, {4}}}) << '\n';
		cout << c.slice({{{.start = 1, .stop = 8, .step = 3}, {.step = 2}}})
				 << '\n';
		cout << c.slice({{{4}, {.step = 2}}}) << '\n';
		cout << c.slice({{{4}, {.step = 2}}}).slice({{{}, {.start = 1, .step = 2}}})
				 << '\n';
		cout
			<< c.slice({{{4}, {.step = 2}}}).slice({{{}, {.start = 1, .step = 2}}})[4]
			<< '\n';

		Tensor<int, 3> d{{5, 5, 5}};
		for (int i{0}; i < 5; i++) {
			for (int j{0}; j < 5; j++) {
				for (int k{0}; k < 5; k++) {
					d[i][j][k] = i * 25 + j * 5 + k;
				}
			}
		}
		cout << d << '\n';
		cout << d[3] << '\n' << '\n';
		cout << d.slice({{{.step = 2}}}) << "\n\n";
		cout << d.slice({{{.step = 2}}}).slice({{{}, {.step = 3}}}) << "\n\n";
		cout << d.slice({{{.step = 2}}})
							.slice({{{}, {.step = 3}}})
							.slice({{{}, {}, {.step = 4}}})
				 << "\n\n";

		Tensor<int, 2> e{{2, 2}, 100, 100, 100, 100};
		cout << c.slice({{{4, 6}, {4, 6}}}) << "\n\n";
		cout << c.slice({{{4, 6}, {4, 6}}}) + e << "\n\n";
		c.slice({{{4, 6}, {4, 6}}}) += e;
		cout << c << "\n\n";
		try {
			c += c.slice({{{4, 6}, {4, 6}}});
			cout << c << "\n\n";
		} catch (Tensor<>::Exception const &exception) {
			cout << exception.what();
		}
	}
	return 0;
}
