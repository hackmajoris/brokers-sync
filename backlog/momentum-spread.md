# Widget: Momentum Spread

## What

Per symbol, the gap between its short-horizon return (1M) and its long-horizon return (10Y annualised or total). Surfaces the two tails: long-term laggards running hot now, and long-term compounders currently out of favour.

## Why

The table has seven performance columns but no way to read them against each other. A symbol that is +12% this month and flat over ten years is a very different proposition from one that is +12% this month on top of a decade of compounding, and today the user has to hold both numbers in their head across a wide horizontal scroll.

## Data

`Position.oneMonthReturn` and `Position.tenYrReturn`. Both already present.

## Implementation notes

- Decide explicitly whether the long-horizon figure is total or annualised, and label it — comparing a total 10Y return against a 1M return is arithmetically meaningless without saying so.
- Symbols younger than the long window will have a null `tenYrReturn`; exclude rather than treating as zero.
- Two short lists ("running hot", "out of favour") reads better than one signed number.

## Effort

Small, but needs a deliberate decision on what the spread actually measures before it is worth shipping.
