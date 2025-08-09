class Solution:
    def maxProduct(self, nums: List[int]) -> int:
        Z = nums[0]
        ZR = (0, 0)
        MX, MN = nums[0], nums[0]
        MXR, MNR = (0, 0), (0, 0)
        for i in range(1, len(nums)):
            S = sorted([(nums[i], (i, i)),
                        (nums[i] * MX, (MXR[0], i)),
                        (nums[i] * MN, (MNR[0], i))])
            MN, MNR = S[0]
            MX, MXR = S[-1]
            if (MX > Z):
                Z, ZR = MX, MXR
        print(ZR)
        return Z
