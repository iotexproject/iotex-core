// SPDX-License-Identifier: MIT
pragma solidity >=0.8;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/security/Pausable.sol";
import "@openzeppelin/contracts/token/ERC721/ERC721.sol";

struct BucketInfo {
    uint256 typeIndex;
    uint256 unlockedAt; // UINT256_MAX: in lock
    uint256 unstakedAt; // UINT256_MAX: in stake
    bytes12 delegate;
}

struct BucketType {
    uint256 amount;
    uint256 duration;
    uint256 activatedAt;
}

contract SystemStaking is ERC721, Ownable, Pausable {
    uint256 public constant UINT256_MAX = type(uint256).max;
    uint256 public constant UNSTAKE_FREEZE_BLOCKS = 51840; // (3 * 24 * 60 * 60) / 5;

    event BucketTypeActivated(uint256 amount, uint256 duration);
    event BucketTypeDeactivated(uint256 amount, uint256 duration);
    event Staked(uint256 indexed tokenId, bytes12 delegate, uint256 amount, uint256 duration);
    event Locked(uint256 indexed tokenId, uint256 duration);
    event Unlocked(uint256 indexed tokenId);
    event Unstaked(uint256 indexed tokenId);
    event Merged(uint256[] indexed tokenIds, uint256 amount, uint256 duration);
    event DurationExtended(uint256 indexed tokenId, uint256 duration);
    event AmountIncreased(uint256 indexed tokenId, uint256 amount);
    event DelegateChanged(uint256 indexed tokenId, bytes12 newDelegate);
    event Withdrawal(uint256 indexed tokenId, address indexed recipient);

    modifier onlyTokenOwner(uint256 _tokenId) {
        _assertOnlyTokenOwner(_tokenId);
        _;
    }

    // token id
    uint256 private __currTokenId;
    // mapping from token ID to bucket
    mapping(uint256 => BucketInfo) private __buckets;
    // delegate name -> bucket type -> count
    mapping(bytes12 => mapping(uint256 => uint256)) private __unlockedVotes;
    // delegate name -> bucket type -> count
    mapping(bytes12 => mapping(uint256 => uint256)) private __lockedVotes;
    // bucket type
    BucketType[] private __bucketTypes;
    // amount -> duration -> index
    mapping(uint256 => mapping(uint256 => uint256)) private __bucketTypeIndices;

    constructor() ERC721("BucketNFT", "BKT") {}

    function pause() external onlyOwner {
        _pause();
    }

    function unpause() external onlyOwner {
        _unpause();
    }

    function unsafeInc(uint256 x) private pure returns (uint256) {
        unchecked {
            return x + 1;
        }
    }

    function unsafeDec(uint256 x) private pure returns (uint256) {
        unchecked {
            return x - 1;
        }
    }

    // bucket type related functions
    function addBucketType(uint256 _amount, uint256 _duration) external onlyOwner {
        require(_amount != 0, "amount is invalid");
        require(__bucketTypeIndices[_amount][_duration] == 0, "duplicate bucket type");
        __bucketTypes.push(BucketType(_amount, _duration, block.number));
        __bucketTypeIndices[_amount][_duration] = __bucketTypes.length;
        emit BucketTypeActivated(_amount, _duration);
    }

    function deactivateBucketType(uint256 _amount, uint256 _duration) external onlyOwner {
        __bucketTypes[_bucketTypeIndex(_amount, _duration)].activatedAt = UINT256_MAX;
        emit BucketTypeDeactivated(_amount, _duration);
    }

    function activateBucketType(uint256 _amount, uint256 _duration) external onlyOwner {
        __bucketTypes[_bucketTypeIndex(_amount, _duration)].activatedAt = block.number;
        emit BucketTypeActivated(_amount, _duration);
    }

    function isActiveBucketType(uint256 _amount, uint256 _duration) external view returns (bool) {
        return _isActiveBucketType(_bucketTypeIndex(_amount, _duration));
    }

    function numOfBucketTypes() public view returns (uint256) {
        return __bucketTypes.length;
    }

    function bucketTypes(
        uint256 _offset,
        uint256 _size
    ) external view returns (BucketType[] memory types_) {
        require(_size > 0 && _offset + _size <= numOfBucketTypes(), "invalid parameters");
        types_ = new BucketType[](_size);
        for (uint256 i = 0; i < _size; i = unsafeInc(i)) {
            unchecked {
                types_[i] = __bucketTypes[_offset + i];
            }
        }
    }

    // token related functions
    function blocksToUnstake(uint256 _tokenId) external view returns (uint256) {
        _assertOnlyValidToken(_tokenId);
        BucketInfo storage bucket = __buckets[_tokenId];
        _assertOnlyStakedToken(bucket);
        return _blocksToUnstake(bucket);
    }

    function blocksToWithdraw(uint256 _tokenId) external view returns (uint256) {
        _assertOnlyValidToken(_tokenId);
        return _blocksToWithdraw(__buckets[_tokenId].unstakedAt);
    }

    function bucketOf(
        uint256 _tokenId
    )
        external
        view
        returns (
            uint256 amount_,
            uint256 duration_,
            uint256 unlockedAt_,
            uint256 unstakedAt_,
            bytes12 delegate_
        )
    {
        _assertOnlyValidToken(_tokenId);
        BucketInfo storage bucket = __buckets[_tokenId];
        BucketType storage bucketType = __bucketTypes[bucket.typeIndex];

        return (
            bucketType.amount,
            bucketType.duration,
            bucket.unlockedAt,
            bucket.unstakedAt,
            bucket.delegate
        );
    }

    function stake(
        uint256 _duration,
        bytes12 _delegate
    ) external payable whenNotPaused returns (uint256) {
        uint256 msgValue = msg.value;
        uint256 index = _bucketTypeIndex(msgValue, _duration);
        _assertOnlyActiveBucketType(index);

        _stake(index, _delegate);
        uint256 tokenId = __currTokenId;
        emit Staked(tokenId, _delegate, msgValue, _duration);

        return tokenId;
    }

    function stake(
        uint256 _amount,
        uint256 _duration,
        bytes12[] memory _delegates
    ) external payable whenNotPaused returns (uint256 firstTokenId_) {
        require(_amount * _delegates.length == msg.value, "invalid parameters");
        uint256 index = _bucketTypeIndex(_amount, _duration);
        _assertOnlyActiveBucketType(index);
        unchecked {
            firstTokenId_ = __currTokenId + 1;
        }
        for (uint256 i = 0; i < _delegates.length; i = unsafeInc(i)) {
            _stake(index, _delegates[i]);
            emit Staked(firstTokenId_ + i, _delegates[i], _amount, _duration);
        }

        return firstTokenId_;
    }

    function stake(
        uint256 _amount,
        uint256 _duration,
        bytes12 _delegate,
        uint256 _count
    ) external payable whenNotPaused returns (uint256 firstTokenId_) {
        require(_count > 0 && _amount * _count == msg.value, "invalid parameters");
        uint256 index = _bucketTypeIndex(_amount, _duration);
        _assertOnlyActiveBucketType(index);
        unchecked {
            firstTokenId_ = __currTokenId + 1;
        }
        for (uint256 i = 0; i < _count; i = unsafeInc(i)) {
            _stake(index, _delegate);
            emit Staked(firstTokenId_ + i, _delegate, _amount, _duration);
        }

        return firstTokenId_;
    }

    function unlock(uint256 _tokenId) external whenNotPaused onlyTokenOwner(_tokenId) {
        BucketInfo storage bucket = __buckets[_tokenId];
        _assertOnlyLockedToken(bucket);
        _unlock(bucket);
        emit Unlocked(_tokenId);
    }

    function unlock(uint256[] calldata _tokenIds) external whenNotPaused {
        uint256 tokenId;
        BucketInfo storage bucket;
        for (uint256 i = 0; i < _tokenIds.length; i = unsafeInc(i)) {
            tokenId = _tokenIds[i];
            _assertOnlyTokenOwner(tokenId);
            bucket = __buckets[tokenId];
            _assertOnlyLockedToken(bucket);
            _unlock(bucket);
            emit Unlocked(tokenId);
        }
    }

    function lock(
        uint256 _tokenId,
        uint256 _duration
    ) external whenNotPaused onlyTokenOwner(_tokenId) {
        BucketInfo storage bucket = __buckets[_tokenId];
        _assertOnlyStakedToken(bucket);
        _lock(bucket, _duration);
        emit Locked(_tokenId, _duration);
    }

    function lock(uint256[] calldata _tokenIds, uint256 _duration) external whenNotPaused {
        uint256 tokenId;
        BucketInfo storage bucket;
        for (uint256 i = 0; i < _tokenIds.length; i = unsafeInc(i)) {
            tokenId = _tokenIds[i];
            _assertOnlyTokenOwner(tokenId);
            bucket = __buckets[tokenId];
            _assertOnlyStakedToken(bucket);
            _lock(bucket, _duration);
            emit Locked(tokenId, _duration);
        }
    }

    function unstake(uint256 _tokenId) external whenNotPaused onlyTokenOwner(_tokenId) {
        BucketInfo storage bucket = __buckets[_tokenId];
        _assertOnlyStakedToken(bucket);
        require(_blocksToUnstake(bucket) == 0, "not ready to unstake");
        _unstake(bucket);
        emit Unstaked(_tokenId);
    }

    function unstake(uint256[] calldata _tokenIds) external whenNotPaused {
        uint256 tokenId;
        BucketInfo storage bucket;
        for (uint256 i = 0; i < _tokenIds.length; i = unsafeInc(i)) {
            tokenId = _tokenIds[i];
            _assertOnlyTokenOwner(tokenId);
            bucket = __buckets[tokenId];
            _assertOnlyStakedToken(bucket);
            require(_blocksToUnstake(bucket) == 0, "not ready to unstake");
            _unstake(bucket);
            emit Unstaked(tokenId);
        }
    }

    function withdraw(
        uint256 _tokenId,
        address payable _recipient
    ) external whenNotPaused onlyTokenOwner(_tokenId) {
        BucketInfo storage bucket = __buckets[_tokenId];
        require(_blocksToWithdraw(bucket.unstakedAt) == 0, "not ready to withdraw");
        _burn(_tokenId);
        _withdraw(bucket, _recipient);
        emit Withdrawal(_tokenId, _recipient);
    }

    function withdraw(
        uint256[] calldata _tokenIds,
        address payable _recipient
    ) external whenNotPaused {
        uint256 tokenId;
        BucketInfo storage bucket;
        for (uint256 i = 0; i < _tokenIds.length; i = unsafeInc(i)) {
            tokenId = _tokenIds[i];
            _assertOnlyTokenOwner(tokenId);
            bucket = __buckets[tokenId];
            require(_blocksToWithdraw(bucket.unstakedAt) == 0, "not ready to withdraw");
            _burn(tokenId);
            _withdraw(bucket, _recipient);
            emit Withdrawal(tokenId, _recipient);
        }
    }

    function merge(
        uint256[] calldata tokenIds,
        uint256 _newDuration
    ) external payable whenNotPaused {
        require(tokenIds.length > 1, "invalid length");
        uint256 amount = msg.value;
        uint256 tokenId;
        BucketInfo storage bucket;
        BucketType storage bucketType;
        for (uint256 i = tokenIds.length; i > 0; ) {
            i = unsafeDec(i);
            tokenId = tokenIds[i];
            _assertOnlyTokenOwner(tokenId);
            bucket = __buckets[tokenId];
            _assertOnlyStakedToken(bucket);
            uint256 typeIndex = bucket.typeIndex;
            bytes12 delegate = bucket.delegate;
            bucketType = __bucketTypes[typeIndex];
            require(_newDuration >= bucketType.duration, "invalid duration");
            amount += bucketType.amount;
            if (_isTriggered(bucket.unlockedAt)) {
                __unlockedVotes[delegate][typeIndex] = unsafeDec(
                    __unlockedVotes[delegate][typeIndex]
                );
            } else {
                __lockedVotes[delegate][typeIndex] = unsafeDec(__lockedVotes[delegate][typeIndex]);
            }
            if (i != 0) {
                _burn(tokenId);
            } else {
                bucket.unlockedAt = UINT256_MAX;
                _updateBucketInfo(bucket, amount, _newDuration);
                emit Merged(tokenIds, amount, _newDuration);
            }
        }
    }

    function extendDuration(
        uint256 _tokenId,
        uint256 _newDuration
    ) external whenNotPaused onlyTokenOwner(_tokenId) {
        BucketInfo storage bucket = __buckets[_tokenId];
        _assertOnlyLockedToken(bucket);
        uint256 typeIndex = bucket.typeIndex;
        BucketType storage bucketType = __bucketTypes[typeIndex];
        require(_newDuration > bucketType.duration, "invalid operation");
        __lockedVotes[bucket.delegate][typeIndex] = unsafeDec(
            __lockedVotes[bucket.delegate][typeIndex]
        );
        _updateBucketInfo(bucket, bucketType.amount, _newDuration);
        emit DurationExtended(_tokenId, _newDuration);
    }

    function increaseAmount(
        uint256 _tokenId,
        uint256 _newAmount
    ) external payable whenNotPaused onlyTokenOwner(_tokenId) {
        BucketInfo storage bucket = __buckets[_tokenId];
        _assertOnlyLockedToken(bucket);
        uint256 typeIndex = bucket.typeIndex;
        BucketType storage bucketType = __bucketTypes[typeIndex];
        require(msg.value + bucketType.amount == _newAmount, "invalid operation");
        __lockedVotes[bucket.delegate][typeIndex] = unsafeDec(
            __lockedVotes[bucket.delegate][typeIndex]
        );
        _updateBucketInfo(bucket, _newAmount, bucketType.duration);
        emit AmountIncreased(_tokenId, _newAmount);
    }

    function changeDelegate(
        uint256 _tokenId,
        bytes12 _delegate
    ) external whenNotPaused onlyTokenOwner(_tokenId) {
        _changeDelegate(__buckets[_tokenId], _delegate);
        emit DelegateChanged(_tokenId, _delegate);
    }

    function changeDelegates(
        uint256[] calldata _tokenIds,
        bytes12 _delegate
    ) external whenNotPaused {
        uint256 tokenId;
        for (uint256 i = 0; i < _tokenIds.length; i = unsafeInc(i)) {
            tokenId = _tokenIds[i];
            _assertOnlyTokenOwner(tokenId);
            _changeDelegate(__buckets[tokenId], _delegate);
            emit DelegateChanged(tokenId, _delegate);
        }
    }

    function lockedVotesTo(
        bytes12[] calldata _delegates
    ) external view returns (uint256[][] memory counts_) {
        counts_ = new uint256[][](_delegates.length);
        uint256 tl = numOfBucketTypes();
        for (uint256 i = 0; i < _delegates.length; i = unsafeInc(i)) {
            counts_[i] = new uint256[](tl);
            mapping(uint256 => uint256) storage votes = __lockedVotes[_delegates[i]];
            for (uint256 j = 0; j < tl; j = unsafeInc(j)) {
                counts_[i][j] = votes[j];
            }
        }

        return counts_;
    }

    function unlockedVotesTo(
        bytes12[] calldata _delegates
    ) external view returns (uint256[][] memory counts_) {
        counts_ = new uint256[][](_delegates.length);
        uint256 tl = numOfBucketTypes();
        for (uint256 i = 0; i < _delegates.length; i = unsafeInc(i)) {
            counts_[i] = new uint256[](tl);
            mapping(uint256 => uint256) storage votes = __unlockedVotes[_delegates[i]];
            for (uint256 j = 0; j < tl; j = unsafeInc(j)) {
                counts_[i][j] = votes[j];
            }
        }

        return counts_;
    }

    /////////////////////////////////////////////
    // Private Functions
    function _bucketTypeIndex(uint256 _amount, uint256 _duration) internal view returns (uint256) {
        uint256 index = __bucketTypeIndices[_amount][_duration];
        require(index > 0, "invalid bucket type");

        return unsafeDec(index);
    }

    // bucket type index `_index` must be valid
    function _isActiveBucketType(uint256 _index) internal view returns (bool) {
        return __bucketTypes[_index].activatedAt <= block.number;
    }

    function _isTriggered(uint256 _value) internal pure returns (bool) {
        return _value != UINT256_MAX;
    }

    function _assertOnlyTokenOwner(uint256 _tokenId) internal view {
        require(msg.sender == ownerOf(_tokenId), "not owner");
    }

    function _assertOnlyLockedToken(BucketInfo storage _bucket) internal view {
        require(!_isTriggered(_bucket.unlockedAt), "not a locked token");
    }

    function _assertOnlyStakedToken(BucketInfo storage _bucket) internal view {
        require(!_isTriggered(_bucket.unstakedAt), "not a staked token");
    }

    function _assertOnlyValidToken(uint256 _tokenId) internal view {
        require(_exists(_tokenId), "ERC721: invalid token ID");
    }

    function _assertOnlyActiveBucketType(uint256 _index) internal view {
        require(_isActiveBucketType(_index), "inactive bucket type");
    }

    function _beforeTokenTransfer(
        address _from,
        address _to,
        uint256 _firstTokenId,
        uint256 _batchSize
    ) internal override {
        require(_batchSize == 1, "batch transfer is not supported");
        require(
            _to == address(0) || !_isTriggered(__buckets[_firstTokenId].unstakedAt),
            "cannot transfer unstaked token"
        );
        super._beforeTokenTransfer(_from, _to, _firstTokenId, _batchSize);
    }

    function _blocksToWithdraw(uint256 _unstakedAt) internal view returns (uint256) {
        require(_isTriggered(_unstakedAt), "not an unstaked bucket");
        uint256 withdrawBlock = _unstakedAt + UNSTAKE_FREEZE_BLOCKS;
        if (withdrawBlock <= block.number) {
            return 0;
        }

        unchecked {
            return withdrawBlock - block.number;
        }
    }

    function _blocksToUnstake(BucketInfo storage _bucket) internal view returns (uint256) {
        uint256 unlockedAt = _bucket.unlockedAt;
        require(_isTriggered(unlockedAt), "not an unlocked bucket");
        uint256 unstakeBlock = unlockedAt + __bucketTypes[_bucket.typeIndex].duration;
        if (unstakeBlock <= block.number) {
            return 0;
        }
        unchecked {
            return unstakeBlock - block.number;
        }
    }

    function _stake(uint256 _index, bytes12 _delegate) internal {
        __currTokenId = unsafeInc(__currTokenId);
        __buckets[__currTokenId] = BucketInfo(_index, UINT256_MAX, UINT256_MAX, _delegate);
        __lockedVotes[_delegate][_index] = unsafeInc(__lockedVotes[_delegate][_index]);
        _safeMint(msg.sender, __currTokenId);
    }

    function _unlock(BucketInfo storage _bucket) internal {
        uint256 typeIndex = _bucket.typeIndex;
        bytes12 delegate = _bucket.delegate;
        _bucket.unlockedAt = block.number;
        __lockedVotes[delegate][typeIndex] = unsafeDec(__lockedVotes[delegate][typeIndex]);
        __unlockedVotes[delegate][typeIndex] = unsafeInc(__unlockedVotes[delegate][typeIndex]);
    }

    function _lock(BucketInfo storage _bucket, uint256 _duration) internal {
        uint256 typeIndex = _bucket.typeIndex;
        bytes12 delegate = _bucket.delegate;
        require(_duration >= _blocksToUnstake(_bucket), "invalid duration");
        uint256 newIndex = _bucketTypeIndex(__bucketTypes[typeIndex].amount, _duration);
        _assertOnlyActiveBucketType(newIndex);
        _bucket.unlockedAt = UINT256_MAX;
        __unlockedVotes[delegate][typeIndex] = unsafeDec(__unlockedVotes[delegate][typeIndex]);
        _bucket.typeIndex = newIndex;
        __lockedVotes[delegate][newIndex] = unsafeInc(__lockedVotes[delegate][newIndex]);
    }

    function _unstake(BucketInfo storage _bucket) internal {
        _bucket.unstakedAt = block.number;
        __unlockedVotes[_bucket.delegate][_bucket.typeIndex] = unsafeDec(
            __unlockedVotes[_bucket.delegate][_bucket.typeIndex]
        );
    }

    function _withdraw(BucketInfo storage _bucket, address payable _recipient) internal {
        uint256 amount = __bucketTypes[_bucket.typeIndex].amount;
        (bool success, ) = _recipient.call{value: amount}("");
        require(success, "failed to transfer");
    }

    function _updateBucketInfo(
        BucketInfo storage _bucket,
        uint256 _amount,
        uint256 _duration
    ) internal {
        uint256 index = _bucketTypeIndex(_amount, _duration);
        _assertOnlyActiveBucketType(index);
        __lockedVotes[_bucket.delegate][index] = unsafeInc(__lockedVotes[_bucket.delegate][index]);
        _bucket.typeIndex = index;
    }

    function _changeDelegate(BucketInfo storage _bucket, bytes12 _newDelegate) internal {
        _assertOnlyStakedToken(_bucket);
        uint256 typeIndex = _bucket.typeIndex;
        bytes12 delegate = _bucket.delegate;
        require(delegate != _newDelegate, "invalid operation");
        if (_isTriggered(_bucket.unlockedAt)) {
            __unlockedVotes[delegate][typeIndex] = unsafeDec(__unlockedVotes[delegate][typeIndex]);
            __unlockedVotes[_newDelegate][typeIndex] = unsafeInc(
                __unlockedVotes[_newDelegate][typeIndex]
            );
        } else {
            __lockedVotes[delegate][typeIndex] = unsafeDec(__lockedVotes[delegate][typeIndex]);
            __lockedVotes[_newDelegate][typeIndex] = unsafeInc(
                __lockedVotes[_newDelegate][typeIndex]
            );
        }
        _bucket.delegate = _newDelegate;
    }
}