package transaction_status_meta_serde_agave

import (
	"encoding/json"
	"fmt"
)

// use serde::{ser::Error, Deserialize, Deserializer, Serialize, Serializer};

// #[derive(Clone, Debug, PartialEq, Eq)]
//
//	pub enum OptionSerializer<T> {
//	    Some(T),
//	    None,
//	    Skip,
//	}
type OptionSerializer[T any] struct {
	some T
	none bool
	skip bool
}

func NewOptionSerializer[T any](value T) *OptionSerializer[T] {
	return &OptionSerializer[T]{}
}

//	impl<T> OptionSerializer<T> {
//	    pub fn none() -> Self {
//	        Self::None
//	    }
func (o *OptionSerializer[T]) None() {
	o.none = true
}

//	pub fn skip() -> Self {
//	    Self::Skip
//	}
func (o *OptionSerializer[T]) Skip() {
	o.skip = true
}

//	pub fn should_skip(&self) -> bool {
//	    matches!(self, Self::Skip)
//	}
func (o *OptionSerializer[T]) ShouldSkip() bool {
	return o.skip
}

//	pub fn or_skip(option: Option<T>) -> Self {
//	    match option {
//	        Option::Some(item) => Self::Some(item),
//	        Option::None => Self::Skip,
//	    }
//	}

//     pub fn as_ref(&self) -> OptionSerializer<&T> {
//         match self {
//             OptionSerializer::Some(item) => OptionSerializer::Some(item),
//             OptionSerializer::None => OptionSerializer::None,
//             OptionSerializer::Skip => OptionSerializer::Skip,
//         }
//     }

//     pub fn as_mut(&mut self) -> OptionSerializer<&mut T> {
//         match *self {
//             OptionSerializer::Some(ref mut x) => OptionSerializer::Some(x),
//             _ => OptionSerializer::None,
//         }
//     }

//	pub fn is_some(&self) -> bool {
//	    matches!(*self, OptionSerializer::Some(_))
//	}
func (o *OptionSerializer[T]) IsSome() bool {
	return !o.none && !o.skip
}

//	pub fn is_none(&self) -> bool {
//	    matches!(*self, OptionSerializer::None)
//	}
func (o *OptionSerializer[T]) IsNone() bool {
	return o.none
}

//	pub fn is_skip(&self) -> bool {
//	    matches!(*self, OptionSerializer::Skip)
//	}
func (o *OptionSerializer[T]) IsSkip() bool {
	return o.skip
}

//	pub fn expect(self, msg: &str) -> T {
//	    match self {
//	        OptionSerializer::Some(val) => val,
//	        _ => panic!("{}", msg),
//	    }
//	}
//
// Expect is a method that returns the value inside the OptionSerializer if it is Some,
// otherwise it panics with the provided message.
func (o *OptionSerializer[T]) Expect(msg string) T {
	if o.IsSome() {
		return o.some
	}
	panic(msg)
}

//	pub fn unwrap(self) -> T {
//	    match self {
//	        OptionSerializer::Some(val) => val,
//	        OptionSerializer::None => {
//	            panic!("called `OptionSerializer::unwrap()` on a `None` value")
//	        }
//	        OptionSerializer::Skip => {
//	            panic!("called `OptionSerializer::unwrap()` on a `Skip` value")
//	        }
//	    }
//	}
func (o *OptionSerializer[T]) Unwrap() T {
	if o.IsSome() {
		return o.some
	}
	if o.IsNone() {
		panic("called `OptionSerializer::unwrap()` on a `None` value")
	}
	if o.IsSkip() {
		panic("called `OptionSerializer::unwrap()` on a `Skip` value")
	}
	panic("called `OptionSerializer::unwrap()` on an unknown value")
}

//	pub fn unwrap_or(self, default: T) -> T {
//	    match self {
//	        OptionSerializer::Some(val) => val,
//	        _ => default,
//	    }
//	}
func (o *OptionSerializer[T]) UnwrapOr(def T) T {
	if o.IsSome() {
		return o.some
	}
	return def
}

// pub fn unwrap_or_else<F>(self, f: F) -> T
// where
//
//	F: FnOnce() -> T,
//
//	{
//	    match self {
//	        OptionSerializer::Some(val) => val,
//	        _ => f(),
//	    }
//	}
func (o *OptionSerializer[T]) UnwrapOrElse(f func() T) T {
	if o.IsSome() {
		return o.some
	}
	return f()
}

//     pub fn map<U, F>(self, f: F) -> Option<U>
//     where
//         F: FnOnce(T) -> U,
//     {
//         match self {
//             OptionSerializer::Some(x) => Some(f(x)),
//             _ => None,
//         }
//     }

//     pub fn map_or<U, F>(self, default: U, f: F) -> U
//     where
//         F: FnOnce(T) -> U,
//     {
//         match self {
//             OptionSerializer::Some(t) => f(t),
//             _ => default,
//         }
//     }

//     pub fn map_or_else<U, D, F>(self, default: D, f: F) -> U
//     where
//         D: FnOnce() -> U,
//         F: FnOnce(T) -> U,
//     {
//         match self {
//             OptionSerializer::Some(t) => f(t),
//             _ => default(),
//         }
//     }

// pub fn filter<P>(self, predicate: P) -> Self
// where
//
//	P: FnOnce(&T) -> bool,
//
//	{
//	    if let OptionSerializer::Some(x) = self {
//	        if predicate(&x) {
//	            return OptionSerializer::Some(x);
//	        }
//	    }
//	    OptionSerializer::None
//	}
func (o *OptionSerializer[T]) Filter(predicate func(T) bool) *OptionSerializer[T] {
	if o.IsSome() {
		if predicate(o.some) {
			return o
		}
	}
	return &OptionSerializer[T]{none: true}
}

//	pub fn ok_or<E>(self, err: E) -> Result<T, E> {
//	    match self {
//	        OptionSerializer::Some(v) => Ok(v),
//	        _ => Err(err),
//	    }
//	}
func (o *OptionSerializer[T]) OkOr(err error) (v T, e error) {
	if o.IsSome() {
		return o.some, nil
	}
	return v, err
}

//     pub fn ok_or_else<E, F>(self, err: F) -> Result<T, E>
//     where
//         F: FnOnce() -> E,
//     {
//         match self {
//             OptionSerializer::Some(v) => Ok(v),
//             _ => Err(err()),
//         }
//     }
// }

// impl<T> From<Option<T>> for OptionSerializer<T> {
//     fn from(option: Option<T>) -> Self {
//         match option {
//             Option::Some(item) => Self::Some(item),
//             Option::None => Self::None,
//         }
//     }
// }

// impl<T> From<OptionSerializer<T>> for Option<T> {
//     fn from(option: OptionSerializer<T>) -> Self {
//         match option {
//             OptionSerializer::Some(item) => Self::Some(item),
//             _ => Self::None,
//         }
//     }
// }

// impl<T: Serialize> Serialize for OptionSerializer<T> {
//     fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
//     where
//         S: Serializer,
//     {
//         match self {
//             Self::Some(item) => item.serialize(serializer),
//             Self::None => serializer.serialize_none(),
//             Self::Skip => Err(Error::custom("Skip variants should not be serialized")),
//         }
//     }
// }

// impl<'de, T: Deserialize<'de>> Deserialize<'de> for OptionSerializer<T> {
//     fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
//     where
//         D: Deserializer<'de>,
//     {
//         Option::deserialize(deserializer).map(Into::into)
//     }
// }

func (o *OptionSerializer[T]) MarshalJSON() ([]byte, error) {
	if o.IsSome() {
		return json.Marshal(o.some)
	}
	if o.IsNone() {
		// TODO: what is the zero value for T?
		return json.Marshal(nil)
	}
	if o.IsSkip() {
		return nil, fmt.Errorf("Skip variants should not be serialized")
	}
	panic("unknown state")
}
