# NaluHandler
개요: H.264/AVC 비디오 압축 표준 NAL을 구성하는 최소 단위 Nalu을 바이트 단위로 삭제하는 프로그램 
(Frame Sceduling Approach for Real-Time Streaming)

---
### 동작 방법
- FFmpeg를 통해 .mp4 파일을 .h264로 변환
- 파일 내 main문 시작의 for문의 start_offset 변수 설정으로 시작 위치 및 삭제 비율 설정 후 실행

---
### 동작 알고리즘
- KMP 알고리즘을 통해서 NAL Unit 시작 패턴 검색
- 시작 패턴은 3byte 패턴(00001)과 4byte 패턴(000001)으로 구성
- go Rutine을 이용해 각 패턴을 탐지하고 반환

---
### 결과물
<p align="">
https://github.com/user-attachments/assets/e48a3fff-27f9-4817-96ef-74f70928510a | https://github.com/user-attachments/assets/9cb745bf-23d8-479a-a733-5ce04e31b11d
---|---|
