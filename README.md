# golangUtil


# JsonUtil

support the mapping type,which is converted to the same as the struct type.you need better put the mapping with struct,or not the map which is converted cant exist the key

### BeachMark

According to the beachmark,the  performance of golangUtil marshal method is better than json.Marshal method.One fifth faster on average
 


# rateLimiterMiddleware


## effect

use the slide windows to limit the  rate of entering the program,which can avoid the influence of burst flow,but also this middleware can promise the  safe of concurrency,and async handler the outdated windows to avoid the performance consume


## warning

According to function test and benchmark test,the rate limiting algorithm is valid in that the window size  greater than 1ms.




### Attribution


the code is still under test,so if the code has any question when you use it,you can connect with the author to correct the coding.Thanks your attribution