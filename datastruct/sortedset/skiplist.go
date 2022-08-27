package sortedset

import "math/rand"

const (
	maxLevel = 16
)

// Element is a key-score pair
type Element struct {
	Member string  //成员
	Score  float64 //分值
}

// Level aspect of a node
type Level struct {
	forward *node // 指向同层的下一个节点
	span    int64 //到forward到跨度
}

type node struct {
	Element
	backward *node    //后向指针
	level    []*Level // 前向指针 level[0]是最下层 因为每个节点每一层指向的节点不同 下标表示层数
}

type skiplist struct {
	header *node
	tail   *node
	length int64
	level  int16
}

func makeNode(level int16, score float64, member string) *node {
	n := &node{
		Element: Element{
			Score:  score,
			Member: member,
		},
		level: make([]*Level, level),
	}
	for i := range n.level {
		n.level[i] = new(Level)
	}
	return n
}

func makeSkiplist() *skiplist {
	return &skiplist{
		level:  1,
		header: makeNode(maxLevel, 0, ""),
	}
}

// randomLevel 随机决定新节点包含的层数
// 1/2->1 1/4->2 1/8->3 因为跳表每两个节点会选取一个节点作为索引
func randomLevel() int16 {
	level := int16(1)
	for float32(rand.Int31()&0xFFFF) < (0.25 * 0xFFFF) {
		level++
	}
	if level < maxLevel {
		return level
	}
	return maxLevel
}

// insert 节点新增
func (skiplist *skiplist) insert(member string, score float64) *node {
	// 保存节点的同层先驱节点
	update := make([]*node, maxLevel)
	// 保存各层先驱节点的排名 用于计算span
	rank := make([]int64, maxLevel)

	node := skiplist.header
	// 层级自上而下寻找 每一层遍历
	for i := skiplist.level - 1; i >= 0; i-- {
		if i == skiplist.level-1 {
			// 最高层作为第0个被寻找
			rank[i] = 0
		} else {
			rank[i] = rank[i+1] // store rank that is crossed to reach the insert position
		}
		// 节点存在前向指针
		if node.level[i] != nil {
			for node.level[i].forward != nil &&
				(node.level[i].forward.Score < score ||
					(node.level[i].forward.Score == score && node.level[i].forward.Member < member)) {
				// 更新新节点第i层第跨度
				rank[i] += node.level[i].span
				// 遍历前向指针节点
				node = node.level[i].forward
			}
		}
		update[i] = node
	}
	// 随机选取层数作为新节点的层级
	level := randomLevel()
	// 如果随机选取节点层级较大 那么超出层级需要更新
	if level > skiplist.level {
		for i := skiplist.level; i < level; i++ {
			rank[i] = 0
			update[i] = skiplist.header
			update[i].level[i].span = skiplist.length
		}
		skiplist.level = level
	}

	// 使用层级/分数/成员新建节点
	node = makeNode(level, score, member)
	for i := int16(0); i < level; i++ {
		node.level[i].forward = update[i].level[i].forward
		update[i].level[i].forward = node

		// update span covered by update[i] as node is inserted here
		node.level[i].span = update[i].level[i].span - (rank[0] - rank[i])
		update[i].level[i].span = (rank[0] - rank[i]) + 1
	}

	// increment span for untouched levels
	for i := level; i < skiplist.level; i++ {
		update[i].level[i].span++
	}

	// 设置后向节点
	if update[0] == skiplist.header {
		node.backward = nil
	} else {
		node.backward = update[0]
	}
	if node.level[0].forward != nil {
		node.level[0].forward.backward = node
	} else {
		skiplist.tail = node
	}
	skiplist.length++
	return node
}

/*
 * param node: 要删除的节点
 * param update: 删除节点的前向节点
 */
func (skiplist *skiplist) removeNode(node *node, update []*node) {
	for i := int16(0); i < skiplist.level; i++ {
		if update[i].level[i].forward == node {
			update[i].level[i].span += node.level[i].span - 1
			update[i].level[i].forward = node.level[i].forward
		} else {
			update[i].level[i].span--
		}
	}
	if node.level[0].forward != nil {
		node.level[0].forward.backward = node.backward
	} else {
		skiplist.tail = node.backward
	}
	for skiplist.level > 1 && skiplist.header.level[skiplist.level-1].forward == nil {
		skiplist.level--
	}
	skiplist.length--
}

/*
 * return: has found and removed node
 */
func (skiplist *skiplist) remove(member string, score float64) bool {
	/*
	 * find backward node (of target) or last node of each level
	 * their forward need to be updated
	 */
	update := make([]*node, maxLevel)
	node := skiplist.header
	for i := skiplist.level - 1; i >= 0; i-- {
		for node.level[i].forward != nil &&
			(node.level[i].forward.Score < score ||
				(node.level[i].forward.Score == score &&
					node.level[i].forward.Member < member)) {
			node = node.level[i].forward
		}
		update[i] = node
	}
	node = node.level[0].forward
	if node != nil && score == node.Score && node.Member == member {
		skiplist.removeNode(node, update)
		// free x
		return true
	}
	return false
}

/*
 * return: 1 based rank, 0 means member not found
 */
func (skiplist *skiplist) getRank(member string, score float64) int64 {
	var rank int64 = 0
	x := skiplist.header
	for i := skiplist.level - 1; i >= 0; i-- {
		for x.level[i].forward != nil &&
			(x.level[i].forward.Score < score ||
				(x.level[i].forward.Score == score &&
					x.level[i].forward.Member <= member)) {
			rank += x.level[i].span
			x = x.level[i].forward
		}

		/* x might be equal to zsl->header, so test if obj is non-NULL */
		if x.Member == member {
			return rank
		}
	}
	return 0
}

/*
 * 1-based rank
 * zrangebyscore key min max 返回指定分数范围内的成员
 */
func (skiplist *skiplist) getByRank(rank int64) *node {
	var i int64 = 0
	n := skiplist.header
	// scan from top level
	for level := skiplist.level - 1; level >= 0; level-- {
		for n.level[level].forward != nil && (i+n.level[level].span) <= rank {
			i += n.level[level].span
			n = n.level[level].forward
		}
		if i == rank {
			return n
		}
	}
	return nil
}

func (skiplist *skiplist) hasInRange(min *ScoreBorder, max *ScoreBorder) bool {
	// min & max = empty
	if min.Value > max.Value || (min.Value == max.Value && (min.Exclude || max.Exclude)) {
		return false
	}
	// min > tail
	n := skiplist.tail
	if n == nil || !min.less(n.Score) {
		return false
	}
	// max < head
	n = skiplist.header.level[0].forward
	if n == nil || !max.greater(n.Score) {
		return false
	}
	return true
}

// getFirstInScoreRange 找到分数范围内的第一个节点
func (skiplist *skiplist) getFirstInScoreRange(min *ScoreBorder, max *ScoreBorder) *node {
	if !skiplist.hasInRange(min, max) {
		return nil
	}
	n := skiplist.header
	// scan from top level
	for level := skiplist.level - 1; level >= 0; level-- {
		// if forward is not in range than move forward
		for n.level[level].forward != nil && !min.less(n.level[level].forward.Score) {
			n = n.level[level].forward
		}
	}
	/* This is an inner range, so the next node cannot be NULL. */
	n = n.level[0].forward
	if !max.greater(n.Score) {
		return nil
	}
	return n
}

// getLastInScoreRange 找到分数范围内最后一个节点
// getLastInScoreRange 找到分数范围内最后一个节点
func (skiplist *skiplist) getLastInScoreRange(min *ScoreBorder, max *ScoreBorder) *node {
	if !skiplist.hasInRange(min, max) {
		return nil
	}
	n := skiplist.header
	// scan from top level
	for level := skiplist.level - 1; level >= 0; level-- {
		for n.level[level].forward != nil && max.greater(n.level[level].forward.Score) {
			n = n.level[level].forward
		}
	}
	if !min.less(n.Score) {
		return nil
	}
	return n
}

/*
 * return removed elements
 */
func (skiplist *skiplist) RemoveRangeByScore(min *ScoreBorder, max *ScoreBorder) (removed []*Element) {
	update := make([]*node, maxLevel)
	removed = make([]*Element, 0)
	// find backward nodes (of target range) or last node of each level
	node := skiplist.header
	for i := skiplist.level - 1; i >= 0; i-- {
		for node.level[i].forward != nil {
			if min.less(node.level[i].forward.Score) { // already in range
				break
			}
			node = node.level[i].forward
		}
		update[i] = node
	}

	// node is the first one within range
	node = node.level[0].forward

	// remove nodes in range
	for node != nil {
		if !max.greater(node.Score) { // already out of range
			break
		}
		next := node.level[0].forward
		removedElement := node.Element
		removed = append(removed, &removedElement)
		skiplist.removeNode(node, update)
		node = next
	}
	return removed
}

// RemoveRangeByRank 1-based rank, including start, exclude stop
func (skiplist *skiplist) RemoveRangeByRank(start int64, stop int64) (removed []*Element) {
	var i int64 = 0 // rank of iterator
	update := make([]*node, maxLevel)
	removed = make([]*Element, 0)

	// 自上而下寻找目标[start,stop]的先驱节点
	node := skiplist.header
	for level := skiplist.level - 1; level >= 0; level-- {
		for node.level[level].forward != nil && (i+node.level[level].span) < start {
			i += node.level[level].span
			node = node.level[level].forward
		}
		update[level] = node
	}

	i++
	node = node.level[0].forward // first node in range

	// remove nodes in range 可能删除不止一个节点
	for node != nil && i < stop {
		next := node.level[0].forward
		removedElement := node.Element
		removed = append(removed, &removedElement)
		skiplist.removeNode(node, update)
		node = next
		i++
	}
	return removed
}
