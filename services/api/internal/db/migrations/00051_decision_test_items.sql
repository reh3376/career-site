-- +goose Up
-- +goose StatementBegin

-- The item bank (docs/decision-test-items-draft.md).
--
-- Thirty scored items and three practice ones, written as data rather
-- than parsed from the draft document. That is deliberate: a regex over
-- prose got 26 of 30 and one of three, and a silent mis-parse here does
-- not break anything visibly, it writes a WRONG ANSWER KEY. Every
-- participant would then be graded against it, once, with no way to
-- re-ask them. The key is the one thing in this project that has to be
-- typed out and read back.
--
-- Four families, all with decent replication records: arithmetic traps,
-- base-rate neglect, conjunction, and belief bias in syllogisms. The
-- circulated classics are excluded (bat-and-ball, widgets, lily pads,
-- Linda) because they would measure prior exposure rather than
-- reflection.
--
-- THE OPTION POSITIONS ARE ROTATED, and that is not cosmetic. Every
-- item was drafted with its correct answer first and its lure second,
-- which in a standardized test is a position habit a participant can
-- learn: by block 3 somebody would be scoring without reading. A
-- deterministic rotation, the same for every participant because the
-- test is standardized, spreads the correct answer across all four
-- positions (8, 11, 11 and 3 of 33).
--
-- The syllogisms carry a standing reminder because this test
-- deliberately degrades the working memory that instructions live in. A
-- participant in block 4 holding four digits and a transformation
-- cannot also be relied on to retain a framing rule given twelve
-- minutes earlier, and without the reminder instruction decay would
-- grow across exactly the blocks where the load effect is measured.
--
-- Presentation order is the insert order, and it matters: the block an
-- item lands in determines the load it is answered under. Families are
-- interleaved rather than blocked so no single block is all arithmetic
-- or all syllogisms.

INSERT INTO dt_items (code, family, kind, prompt, reminder, options, correct_index, lure_index, rationale)
VALUES
  ('A1','arithmetic','scored','A notebook and a pencil cost $5.40 together. The notebook costs $5.00 more than the pencil. How much is the pencil?','','["$0.20", "$0.40", "$0.45", "$1.08"]'::jsonb,0,1,'Subtracting instead of solving for two unknowns.'),
  ('A2','arithmetic','scored','Six machines take 6 minutes to make 6 parts. How long would 100 machines take to make 100 parts?','','["1 minute", "6 minutes", "100 minutes", "60 minutes"]'::jsonb,1,2,'Matching the numbers instead of finding the rate.'),
  ('A3','arithmetic','scored','Algae on a pond doubles every day and covers the whole pond on day 30. On which day was the pond half covered?','','["Day 28", "Day 16", "Day 29", "Day 15"]'::jsonb,2,3,'Halving the endpoint instead of stepping back one doubling.'),
  ('A4','arithmetic','scored','A jacket costs $80. It is cut by 25% in a sale, then the sale price is raised by 25% the following week. What does it cost now?','','["$80", "$85", "$70", "$75"]'::jsonb,3,0,'Assuming equal percentages cancel, when they apply to different bases.'),
  ('A5','arithmetic','scored','You drive 60 miles at 30 mph, then the same 60 miles back at 60 mph. What is your average speed for the trip?','','["40 mph", "45 mph", "50 mph", "30 mph"]'::jsonb,0,1,'Averaging the speeds rather than dividing distance by total time.'),
  ('A6','arithmetic','scored','A shirt is marked down 40%, then a further 20% comes off at the till. What is the total discount?','','["56%", "52%", "60%", "48%"]'::jsonb,1,2,'Adding the percentages.'),
  ('A7','arithmetic','scored','A train covers 90 miles in 90 minutes. How far does it travel in one hour?','','["75 miles", "45 miles", "60 miles", "90 miles"]'::jsonb,2,3,'The matching numbers invite the answer before the units are read.'),
  ('A8','arithmetic','scored','If 5 painters take 5 hours to paint 5 rooms, how long does 1 painter take to paint 1 room?','','["1 hour", "25 hours", "half an hour", "5 hours"]'::jsonb,3,0,'Dividing through, when the per-painter rate is unchanged.'),
  ('A9','arithmetic','scored','A bag of flour weighs 3 kg plus half its own weight. What does it weigh?','','["6 kg", "4.5 kg", "3 kg", "9 kg"]'::jsonb,0,1,'Adding half of the stated 3 kg rather than solving W = 3 + W/2.'),
  ('A10','arithmetic','scored','A clock takes 6 seconds to strike 4 o''clock. How long does it take to strike 8 o''clock?','','["8 seconds", "14 seconds", "12 seconds", "16 seconds"]'::jsonb,1,2,'Doubling the time with the hour. The gaps take the time, and there are three at four o''clock.'),
  ('B1','base_rate','scored','One person in 100 has a condition. A test catches 90% of those who have it, and wrongly flags 10% of those who do not. Someone tests positive. Roughly what is the chance they have it?','','["About 50%", "About 10%", "About 8%", "About 90%"]'::jsonb,2,3,'Reading the test''s accuracy as the answer and dropping the base rate. True value 8.3%.'),
  ('B2','base_rate','scored','A conference has 80 nurses and 20 surgeons. You meet someone decisive who enjoys pressure. Which is more likely?','','["They are a surgeon", "Equally likely", "They are a nurse"]'::jsonb,2,0,'A weak stereotype overriding a 4:1 base rate.'),
  ('B3','base_rate','scored','Which gives the better chance of winning: drawing one winning ticket from a bowl of 10, or nine winning tickets from a bowl of 100?','','["One from 10", "Nine from 100", "The same"]'::jsonb,0,1,'The larger number of winners feels like better odds. 10% against 9%.'),
  ('B4','base_rate','scored','One bag holds 10 red marbles and 90 blue. Another holds 1 red and 4 blue. Which gives the better chance of drawing red?','','["The same", "The second bag", "The first bag"]'::jsonb,1,2,'Ten reds feels like more chance than one. 20% against 10%.'),
  ('B5','base_rate','scored','A condition affects 1 person in 10,000. A test is right 99% of the time. Someone tests positive. Roughly what chance do they have it?','','["About 50%", "About 90%", "About 1%", "About 99%"]'::jsonb,2,3,'The same trap at a rarer base rate, where the gap is starker. True value 0.98%.'),
  ('B6','base_rate','scored','A depot runs 85 white vans and 15 red. A witness says the van was red, and is right 80% of the time. How likely is it the van was red?','','["About 80%", "About 15%", "About 60%", "About 40%"]'::jsonb,3,0,'Taking the witness''s reliability as the answer. True value 41%.'),
  ('B7','base_rate','scored','One polling station counts about 500 ballots a day, another about 50. Which is more likely to report a day where over 60% went one way?','','["Equally likely", "The smaller station", "The larger station"]'::jsonb,1,2,'Small samples swing further. Size feels like it should produce extremes rather than damp them.'),
  ('B8','base_rate','scored','Of 1,000 people tested, 5 have a condition. The test flags all 5 of them, and also flags 95 who do not have it. Someone is flagged. What is the chance they have it?','','["95%", "5%", "100%", "50%"]'::jsonb,1,2,'''Flags all 5'' reads as a perfect test. The easier natural-frequency framing, as a contrast against B1 and B5.'),
  ('C1','conjunction','scored','Dan reads two papers a day and argues about politics. Which is more likely?','','["Dan is a teacher", "Dan is a teacher who attends council meetings", "Equally likely"]'::jsonb,0,1,'The detailed version fits the description and is a subset of the first.'),
  ('C2','conjunction','scored','Maya studied biology and volunteers at an animal shelter. Which is more likely?','','["Equally likely", "Maya works in finance", "Maya works in finance and fosters animals"]'::jsonb,1,2,'The second fits the description and is contained by the first.'),
  ('C3','conjunction','scored','Which is more likely in the next five years?','','["A major flood in Europe caused by a dam failure", "Equally likely", "A major flood somewhere in Europe"]'::jsonb,2,0,'A cause makes an event easier to picture and cannot make it more likely.'),
  ('C4','conjunction','scored','Tom is 34, methodical, and keeps detailed records of everything. Which is more likely?','','["Tom plays an instrument", "Tom plays an instrument and practises every day", "Equally likely"]'::jsonb,0,1,'Daily practice fits methodical. It is still a subset.'),
  ('D1','syllogism','scored','All roses are flowers. Some flowers fade quickly. Therefore some roses fade quickly. Assuming both statements are true, does the conclusion follow?','Assume both statements are true.','["Cannot tell", "No", "Yes"]'::jsonb,1,2,'Believable and invalid. The flowers that fade need not be roses.'),
  ('D2','syllogism','scored','No experienced drivers are reckless. Some reckless people are young. Therefore some young people are not experienced drivers. Assuming both statements are true, does the conclusion follow?','Assume both statements are true.','["No", "Cannot tell", "Yes"]'::jsonb,2,0,'Valid but awkward, so it feels wrong.'),
  ('D3','syllogism','scored','All nurses are trained in first aid. Some people trained in first aid work in schools. Therefore some nurses work in schools. Assuming both statements are true, does the conclusion follow?','Assume both statements are true.','["No", "Yes", "Cannot tell"]'::jsonb,0,1,'Believable and invalid. The school workers need not be the nurses.'),
  ('D4','syllogism','scored','All metals float. Gold is a metal. Therefore gold floats. Assuming both statements are true, does the conclusion follow?','Assume both statements are true.','["Cannot tell", "Yes", "No"]'::jsonb,1,2,'Valid, with a false premise and an absurd conclusion. Validity is about the argument, not the world.'),
  ('D5','syllogism','scored','No birds are insects. All sparrows are birds. Therefore no sparrows are insects. Assuming both statements are true, does the conclusion follow?','Assume both statements are true.','["No", "Cannot tell", "Yes"]'::jsonb,2,0,'Valid and believable. A control: feel and logic agree.'),
  ('D6','syllogism','scored','All doctors have a degree. Some doctors work nights. Therefore some people who work nights have a degree. Assuming both statements are true, does the conclusion follow?','Assume both statements are true.','["Yes", "No", "Cannot tell"]'::jsonb,0,1,'Valid and believable, with enough clauses to need reading.'),
  ('D7','syllogism','scored','All vegetables are grown in soil. Some things grown in soil are poisonous. Therefore all vegetables are poisonous. Assuming both statements are true, does the conclusion follow?','Assume both statements are true.','["Cannot tell", "No", "Yes"]'::jsonb,1,2,'Invalid and absurd. A control in the other direction.'),
  ('D8','syllogism','scored','Some teachers are novelists. All novelists are wealthy. Therefore all teachers are wealthy. Assuming both statements are true, does the conclusion follow?','Assume both statements are true.','["Yes", "Cannot tell", "No"]'::jsonb,2,0,'Invalid: only ''some teachers are wealthy'' follows.'),
  ('P1','arithmetic','practice','A coffee and a muffin cost $7.50 together. The coffee costs $6.50 more than the muffin. How much is the muffin?','','["$0.75", "$1.50", "$0.50", "$1.00"]'::jsonb,2,3,'Practice. Same trap as A1 at different numbers.'),
  ('P2','syllogism','practice','All birds are made of glass. A sparrow is a bird. Therefore a sparrow is made of glass. Assuming both statements are true, does the conclusion follow?','Assume both statements are true.','["Cannot tell", "Yes", "No"]'::jsonb,1,2,'Practice, and it teaches the framing: valid with an absurd conclusion.'),
  ('P3','base_rate','practice','Which is the better chance: 2 winning tickets from 20, or 7 from 100?','','["7 from 100", "The same", "2 from 20"]'::jsonb,2,0,'Practice. 10% against 7%.')
ON CONFLICT (tenant_id, code, version) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM dt_items WHERE version = 1;
-- +goose StatementEnd
